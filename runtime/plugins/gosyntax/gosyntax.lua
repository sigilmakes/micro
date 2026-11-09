VERSION = "0.1.0"

local syntax = import("micro/syntax")

local groups = {
    ["break"] = "statement",
    ["case"] = "statement",
    ["continue"] = "statement",
    ["default"] = "statement",
    ["else"] = "statement",
    ["fallthrough"] = "statement",
    ["for"] = "statement",
    ["goto"] = "statement",
    ["if"] = "statement",
    ["range"] = "statement",
    ["return"] = "statement",
    ["select"] = "statement",
    ["switch"] = "statement",

    ["package"] = "preproc",
    ["import"] = "preproc",
    ["const"] = "preproc",
    ["var"] = "preproc",
    ["type"] = "preproc",
    ["struct"] = "preproc",
    ["interface"] = "preproc",
    ["func"] = "preproc",
    ["map"] = "preproc",
    ["chan"] = "preproc",
    ["go"] = "preproc",
    ["defer"] = "preproc",

    ["true"] = "constant.bool",
    ["false"] = "constant.bool",
    ["nil"] = "constant.bool",
    ["iota"] = "constant.bool",

    ["bool"] = "type",
    ["byte"] = "type",
    ["complex64"] = "type",
    ["complex128"] = "type",
    ["error"] = "type",
    ["float32"] = "type",
    ["float64"] = "type",
    ["int"] = "type",
    ["int8"] = "type",
    ["int16"] = "type",
    ["int32"] = "type",
    ["int64"] = "type",
    ["rune"] = "type",
    ["string"] = "type",
    ["uint"] = "type",
    ["uint8"] = "type",
    ["uint16"] = "type",
    ["uint32"] = "type",
    ["uint64"] = "type",
    ["uintptr"] = "type",
}

local function add(out, n, line, first, last, group)
    out[n + 1] = line
    out[n + 2] = first
    out[n + 3] = last
    out[n + 4] = group
    return n + 4
end

local function find_comment(line)
    local i = 1
    while i <= #line do
        local c = line:sub(i, i)
        if c == '"' then
            i = i + 1
            while i <= #line do
                local q = line:sub(i, i)
                if q == "\\" then
                    i = i + 2
                elseif q == '"' then
                    i = i + 1
                    break
                else
                    i = i + 1
                end
            end
        elseif c == "`" then
            local e = line:find("`", i + 1, true)
            if not e then
                break
            end
            i = e + 1
        elseif c == "/" and line:sub(i + 1, i + 1) == "/" then
            return i
        else
            i = i + 1
        end
    end
    return nil
end

local function scan_quoted(code, quote, line, out, n)
    local i = 1
    while true do
        local first = code:find(quote, i, true)
        if not first then
            return n
        end
        local last = code:find(quote, first + 1, true)
        if last then
            n = add(out, n, line, first - 1, last, "constant.string")
            i = last + 1
        else
            n = add(out, n, line, first - 1, #code, "constant.string")
            return n
        end
    end
end

local function highlight_line(line, lineNo, out, n)
    local comment = find_comment(line)
    local code = comment and line:sub(1, comment - 1) or line

    n = scan_quoted(code, '"', lineNo, out, n)
    n = scan_quoted(code, "`", lineNo, out, n)

    for first, last in code:gmatch("()%d+()") do
        local before = first > 1 and code:sub(first - 1, first - 1) or ""
        if not before:match("[%a_]") then
            n = add(out, n, lineNo, first - 1, last - 1, "constant.number")
        end
    end

    for first, last, word in code:gmatch("()([%a_][%w_]*)()") do
        local group = groups[word]
        if group then
            n = add(out, n, lineNo, first - 1, last - 1, group)
        end
    end

    if comment then
        n = add(out, n, lineNo, comment - 1, #line, "comment")
    end
    return n
end

local function request(ctx, reply)
    local out, n = {}, 0
    local lineNo = 0
    for line in (ctx.text .. "\n"):gmatch("(.-)\n") do
        if lineNo > ctx.last then
            break
        end
        if lineNo >= ctx.first then
            n = highlight_line(line, lineNo, out, n)
        end
        lineNo = lineNo + 1
    end
    reply(out)
end

function init()
    syntax.register {
        name = "go-hardcoded",
        filetype = "go",
        filename = "\\.go$",
        priority = -1000,
        request = request,
    }
end
