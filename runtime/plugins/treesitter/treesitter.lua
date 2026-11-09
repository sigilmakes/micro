VERSION = "0.2.0"

-- Tree-sitter highlighting for the filetypes linked into the helper.
-- The helper owns parsing, grammars, queries, and capture mapping; this
-- plugin owns the process, request framing, and reply generation checks.
--
-- One provider is registered per filetype. They all share one helper process
-- and one request function, so a buffer switch does not restart the helper.

local syntax = import("micro/syntax")
local shell = import("micro/shell")
local micro = import("micro")
local os = import("os")

-- Mirrors pkg/filetype/table.go for micro's tree-sitter filetypes.
local PROVIDERS = {
    { filetype = "go", filename = [[\.go$]] },
    { filetype = "python", filename = [[\.py(3|w)?$]] },
    { filetype = "javascript", filename = [[(\.(m|c)?js$|\.es[5678]?$)]] },
    { filetype = "typescript", filename = [[\.tsx?$]] },
    { filetype = "rust", filename = [[\.rs$]] },
    { filetype = "c", filename = [[(\.(c|C)$|\.(h|H)$|\.ii?$|\.(def)$)]] },
    {
        filetype = "c++",
        filename = [[(\.c(c|pp|xx)$|\.h(h|pp|xx)?$|\.ii?$|\.(def)$)]],
        signature = [[\b(namespace|class|public|protected|private|template|constexpr|noexcept|nullptr|throw)\b]],
    },
    {
        filetype = "shell",
        filename = [[(\.(sh|bash|ash|ebuild)$|(\.bash(rc|_aliases|_functions|_profile)|\.?profile|Pkgfile|pkgmk\.conf|rc\.conf|PKGBUILD|APKBUILD)$|bash-fc\.)]],
    },
    { filetype = "json", filename = [[\.jsonc?$]] },
    { filetype = "toml", filename = [[\.toml$]] },
    { filetype = "html", filename = [[\.htm[l]?$]] },
    { filetype = "css", filename = [[\.(css|scss)$]] },
    { filetype = "lua", filename = [[\.lua$]] },
    { filetype = "yaml", filename = [[\.ya?ml$]] },
}

local job = nil
local disabled = false
local logged = false
local providersRegistered = false
local spawnGeneration = 0
local helperPath = nil

local outputBuffer = ""
local collecting = nil
local replies = {}
local bufferIDs = {}
local nextBufferID = 0
local errorLogged = {}

local function logOnce(message)
    if not logged then
        logged = true
        micro.Log("treesitter: " .. message)
    end
end

local function nextLine()
    local newline = outputBuffer:find("\n", 1, true)
    if not newline then
        return nil
    end
    local line = outputBuffer:sub(1, newline - 1)
    outputBuffer = outputBuffer:sub(newline + 1)
    return line
end

local function deliver(id, generation, spans, errMessage)
    local pending = replies[id]
    if pending == nil then
        return
    end
    if tostring(pending.generation) ~= tostring(generation) then
        return
    end
    replies[id] = nil
    if errMessage ~= nil then
        local filetype = pending.filetype or ""
        if not errorLogged[filetype] then
            errorLogged[filetype] = true
            micro.Log("treesitter: " .. filetype .. " unavailable: " .. errMessage)
        end
        -- A failed request carries no spans. Clear the previous ones so the
        -- buffer is never coloured from a stale generation.
        pcall(pending.reply, {})
        return
    end
    local ok, err = pcall(pending.reply, spans)
    if not ok then
        micro.Log("treesitter: reply failed: " .. tostring(err))
    end
end

local function stopJob(generation)
    if generation ~= spawnGeneration then
        return
    end
    spawnGeneration = spawnGeneration + 1
    if job ~= nil then
        shell.JobStop(job)
        job = nil
    end
    replies = {}
    collecting = nil
    outputBuffer = ""
end

local function protocolFailure(generation, message)
    if generation ~= spawnGeneration then
        return
    end
    logOnce("helper protocol error: " .. message)
    disabled = true
    stopJob(generation)
end

local process -- forward declaration

local function handleHeader(generation, line)
    local kind, id, gen, rest = line:match("^(%S+)\t(%S+)\t(%S+)\t(.*)$")
    if kind == nil then
        protocolFailure(generation, "malformed header: " .. line)
        return
    end
    if kind == "SPANS" then
        local count = tonumber(rest)
        if count == nil or count < 0 or count ~= math.floor(count) then
            protocolFailure(generation, "bad span count: " .. rest)
            return
        end
        if count == 0 then
            deliver(id, gen, {})
            return
        end
        collecting = { id = id, generation = gen, count = count, spans = {} }
    elseif kind == "ERR" then
        deliver(id, gen, nil, rest)
    else
        protocolFailure(generation, "unknown frame: " .. kind)
    end
end

local function handleSpanLine(generation, line)
    local lineNo, startByte, endByte, group =
        line:match("^(%-?%d+)\t(%-?%d+)\t(%-?%d+)\t(.*)$")
    if lineNo == nil then
        protocolFailure(generation, "malformed span line: " .. line)
        return
    end
    local spans = collecting.spans
    local n = #spans
    spans[n + 1] = tonumber(lineNo)
    spans[n + 2] = tonumber(startByte)
    spans[n + 3] = tonumber(endByte)
    spans[n + 4] = group
    if #spans >= collecting.count * 4 then
        local finished = collecting
        collecting = nil
        deliver(finished.id, finished.generation, finished.spans)
    end
end

process = function(generation)
    if generation ~= spawnGeneration then
        return
    end
    while true do
        local line = nextLine()
        if line == nil then
            return
        end
        if collecting ~= nil then
            handleSpanLine(generation, line)
        else
            handleHeader(generation, line)
        end
        if generation ~= spawnGeneration then
            return
        end
    end
end

local function bufferID(ctx)
    if ctx.id ~= nil then
        return tostring(ctx.id)
    end
    -- Older core builds may omit ctx.id. Derive a stable numeric id from the
    -- buffer handle so the helper can still key its parse state.
    local key = tostring(ctx.buf)
    local id = bufferIDs[key]
    if id == nil then
        nextBufferID = nextBufferID + 1
        id = nextBufferID
        bufferIDs[key] = id
    end
    return tostring(id)
end

local function request(ctx, reply)
    if disabled or job == nil then
        reply({})
        return
    end
    local id = bufferID(ctx)
    local generation = ctx.generation or 0
    local filetype = ctx.filetype or ""
    local text = ctx.text or ""
    replies[id] = { generation = generation, reply = reply, filetype = filetype }
    local frame = "REQ\t" .. id .. "\t" .. tostring(generation) .. "\t"
        .. filetype .. "\t" .. tostring(#text) .. "\n" .. text
    shell.JobSend(job, frame)
end

local function startHelper()
    spawnGeneration = (spawnGeneration or 0) + 1
    local generation = spawnGeneration
    outputBuffer = ""
    collecting = nil
    replies = {}

    local function onStdout(data, args)
        if generation ~= spawnGeneration then
            return
        end
        outputBuffer = outputBuffer .. data
        process(generation)
    end

    local function onStderr(data, args)
        if generation ~= spawnGeneration then
            return
        end
        local detail = tostring(data or "")
        if #detail > 400 then
            detail = detail:sub(1, 400) .. "..."
        end
        micro.Log("treesitter: " .. detail)
    end

    local function onExit(output, args)
        if generation ~= spawnGeneration then
            return
        end
        job = nil
        disabled = true
        collecting = nil
        outputBuffer = ""
        replies = {}
        local detail = tostring(output or "")
        if #detail > 400 then
            detail = detail:sub(1, 400) .. "..."
        end
        detail = detail:gsub("%s+$", "")
        if detail ~= "" then
            logOnce("helper exited (" .. helperPath .. "): " .. detail)
        else
            logOnce("could not run helper " .. helperPath .. " (set MICRO_TS or install micro-treesitter)")
        end
    end

    job = shell.JobSpawn(helperPath, {}, onStdout, onStderr, onExit)
end

local function registerProviders()
    local failed = false
    for _, spec in ipairs(PROVIDERS) do
        local ok, err = pcall(syntax.register, {
            name = "treesitter-" .. spec.filetype,
            filetype = spec.filetype,
            filename = spec.filename,
            signature = spec.signature,
            priority = 100,
            request = request,
        })
        if not ok then
            failed = true
            micro.Log("treesitter: register " .. spec.filetype .. ": " .. tostring(err))
        end
    end
    if failed then
        disabled = true
        if job ~= nil then
            stopJob(spawnGeneration)
        end
    else
        providersRegistered = true
    end
end

function init()
    if syntax == nil then
        logOnce("this micro build has no micro/syntax provider API")
        disabled = true
        return
    end
    if job ~= nil then
        return
    end

    disabled = false
    logged = false
    errorLogged = {}
    helperPath = os.Getenv("MICRO_TS")
    if helperPath == nil or helperPath == "" then
        helperPath = "micro-treesitter"
    end

    startHelper()

    if not providersRegistered then
        registerProviders()
    end
end

function deinit()
    spawnGeneration = spawnGeneration + 1
    if job ~= nil then
        shell.JobStop(job)
        job = nil
    end
    outputBuffer = ""
    collecting = nil
    replies = {}
    bufferIDs = {}
    nextBufferID = 0
    disabled = false
    logged = false
    providersRegistered = false
    errorLogged = {}
end
