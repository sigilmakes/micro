-- Standalone harness for runtime/plugins/treesitter/treesitter.lua.
-- Run from the repository root: lua5_1 testdata/lua_harness.lua
--
-- It mocks micro's Lua modules, drives the plugin's protocol state machine
-- with chunked helper output, and asserts the reply/generation behaviour.

local logs = {}
local sent = {}
local stops = 0
local registered
local onStdout, onStderr, onExit

function import(name)
    if name == "micro/syntax" then
        return { register = function(provider) registered = provider end }
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
        return { getenv = function() return nil end }
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

init()
check(registered ~= nil, "provider was not registered")
check(registered.name == "treesitter-go", "provider name")
check(registered.filetype == "go", "provider filetype")
check(registered.filename == "\\.go$", "provider filename regex")
check(registered.priority == 100, "provider priority")

-- Frame framing: exact bytes, byte length, id and generation from ctx.
local delivered
local function reply(spans) delivered = spans end
local text = "package x\n"
registered.request({ id = 1, generation = 5, filetype = "go", text = text }, reply)
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
registered.request({ id = 1, generation = 6, filetype = "go", text = text }, reply)
feed("SPANS\t1\t5\t0\n")
check(delivered == nil, "stale generation was delivered")
feed("SPANS\t1\t6\t0\n")
check(delivered ~= nil and #delivered == 0, "current generation clears spans")

-- ERR for the current generation clears spans and logs once.
delivered = nil
registered.request({ id = 1, generation = 7, filetype = "go", text = text }, reply)
feed("ERR\t1\t7\tquery failed\n")
check(delivered ~= nil and #delivered == 0, "ERR should clear spans")
local sawError = false
for _, line in ipairs(logs) do
    if line:find("query failed", 1, true) then sawError = true end
end
check(sawError, "ERR was not logged")

-- Unknown frames are a protocol failure: the helper is stopped and disabled.
local stopsBefore = stops
feed("WHAT\t1\t7\tnonsense\n")
check(stops == stopsBefore + 1, "unknown frame did not stop the helper")
delivered = nil
registered.request({ id = 1, generation = 8, filetype = "go", text = text }, reply)
check(delivered ~= nil and #delivered == 0, "disabled plugin must still answer")

deinit()
check(stops == stopsBefore + 1, "deinit stopped an already stopped helper")

print("lua harness: ok")
