-- Standalone harness for runtime/plugins/treesitter/treesitter.lua.
-- Run from the repository root: lua5_1 testdata/lua_harness.lua
--
-- It mocks micro's Lua modules, drives the plugin's protocol state machine
-- with chunked helper output, and asserts the reply/generation behaviour.

local logs = {}
local sent = {}
local stops = 0
local registered = {}
local onStdout, onStderr, onExit

function import(name)
    if name == "micro/syntax" then
        return { register = function(provider) registered[#registered + 1] = provider end }
    elseif name == "micro/shell" then
        return {
            JobSpawn = function(command, args, out, err, exit)
                onStdout, onStderr, onExit = out, err, exit
                return { command = command }
            end,
            JobSend = function(job, data) sent[#sent + 1] = data end,
            JobStop = function(job) stops = stops + 1 end,
        }
    elseif name == "micro" then
        return { Log = function(...) logs[#logs + 1] = table.concat({ ... }, " ") end }
    elseif name == "os" then
        return { Getenv = function() return nil end }
    end
    error("unexpected import: " .. name)
end

dofile("runtime/plugins/treesitter/treesitter.lua")

local function check(condition, message)
    if not condition then
        error(message, 2)
    end
end

local function feed(chunk)
    onStdout(chunk, {})
end

local function joinFlat(spans)
    local parts = {}
    for i = 1, #spans do
        parts[i] = tostring(spans[i])
    end
    return table.concat(parts, "|")
end

local function providerFor(filetype)
    for _, provider in ipairs(registered) do
        if provider.filetype == filetype then
            return provider
        end
    end
    return nil
end

local function countLogs(fragment)
    local n = 0
    for _, line in ipairs(logs) do
        if line:find(fragment, 1, true) then n = n + 1 end
    end
    return n
end

init()
check(#registered == 14, "expected 14 providers, got " .. #registered)
local go = providerFor("go")
check(go ~= nil, "go provider was not registered")
check(go.name == "treesitter-go", "provider name")
check(go.filename == "\\.go$", "provider filename regex")
check(go.priority == 100, "provider priority")
for _, filetype in ipairs({ "go", "python", "javascript", "typescript", "rust", "c",
    "c++", "shell", "json", "toml", "html", "css", "lua", "yaml" }) do
    local provider = providerFor(filetype)
    check(provider ~= nil, "missing provider for " .. filetype)
    check(provider.name == "treesitter-" .. filetype, "wrong name for " .. filetype)
    check(provider.request == go.request, "provider request function is not shared")
end
check(providerFor("c++").signature ~= nil, "c++ signature missing")

-- Frame framing: exact bytes, byte length, id and generation from ctx.
local delivered
local function reply(spans) delivered = spans end
local text = "package x\n"
go.request({ id = 1, generation = 5, filetype = "go", text = text }, reply)
check(#sent == 1, "expected one REQ frame, got " .. #sent)
local wantFrame = "REQ\t1\t5\tgo\t10\n" .. text
check(sent[1] == wantFrame, string.format("frame mismatch: %q", sent[1]))

-- Chunked reply: header and spans split at arbitrary byte boundaries.
feed("SPA")
check(delivered == nil, "replied before the header was complete")
feed("NS\t1\t5\t2\n0\t0\t7\tprepr")
check(delivered == nil, "replied before the first span line was complete")
feed("oc\n1\t0\t1\tcomment\n")
check(delivered ~= nil, "reply was not delivered")
check(joinFlat(delivered) == "0|0|7|preproc|1|0|1|comment",
    "span payload: " .. joinFlat(delivered))

-- A stale generation must not overwrite the newer pending reply.
delivered = nil
go.request({ id = 1, generation = 6, filetype = "go", text = text }, reply)
feed("SPANS\t1\t5\t0\n")
check(delivered == nil, "stale generation was delivered")
feed("SPANS\t1\t6\t0\n")
check(delivered ~= nil and #delivered == 0, "current generation clears spans")

-- ERR for the current generation clears spans and logs once per filetype.
delivered = nil
go.request({ id = 1, generation = 7, filetype = "ruby", text = "puts 1\n" }, reply)
feed("ERR\t1\t7\tunsupported filetype \"ruby\"\n")
check(delivered ~= nil and #delivered == 0, "ERR should clear spans")
check(countLogs("ruby unavailable") == 1, "ERR was not logged exactly once")
delivered = nil
go.request({ id = 1, generation = 8, filetype = "ruby", text = "puts 1\n" }, reply)
feed("ERR\t1\t8\tunsupported filetype \"ruby\"\n")
check(countLogs("ruby unavailable") == 1, "repeated ERR was logged again")

-- Unknown frames are a protocol failure: the helper is stopped and disabled.
local stopsBefore = stops
feed("WHAT\t1\t8\tnonsense\n")
check(stops == stopsBefore + 1, "unknown frame did not stop the helper")
delivered = nil
go.request({ id = 1, generation = 9, filetype = "go", text = text }, reply)
check(delivered ~= nil and #delivered == 0, "disabled plugin must still answer")

deinit()
check(stops == stopsBefore + 1, "deinit stopped an already stopped helper")

print("lua harness: ok")
