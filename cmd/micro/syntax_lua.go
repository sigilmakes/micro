package main

import (
	"log"
	"regexp"
	"strings"

	lua "github.com/yuin/gopher-lua"
	luar "layeh.com/gopher-luar"

	"github.com/micro-editor/micro/v2/internal/buffer"
	"github.com/micro-editor/micro/v2/internal/config"
	ulua "github.com/micro-editor/micro/v2/internal/lua"
	"github.com/micro-editor/micro/v2/internal/screen"
	"github.com/micro-editor/micro/v2/pkg/syntax"
)

func luaImportMicroSyntax() *lua.LTable {
	L := ulua.L
	pkg := L.NewTable()

	L.SetField(pkg, "register", L.NewFunction(func(L *lua.LState) int {
		t := L.CheckTable(1)
		p, err := providerFromTable(L, t, ulua.Loading)
		if err != nil {
			screen.TermMessage("syntax.register: " + err.Error())
			return 0
		}
		syntax.Register(p)
		buffer.RefreshSyntax()
		return 0
	}))

	L.SetField(pkg, "unregister", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		full := namespaced(ulua.Loading, name)
		syntax.Unregister(full)
		if full != name {
			syntax.Unregister(name)
		}
		buffer.RefreshSyntax()
		return 0
	}))

	L.SetField(pkg, "invalidate", L.NewFunction(func(L *lua.LState) int {
		b := bufferFromLua(L, 1)
		b.InvalidateSyntax()
		return 0
	}))

	L.SetField(pkg, "has_color", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		L.Push(lua.LBool(hasColor(name)))
		return 1
	}))

	L.SetField(pkg, "providers", L.NewFunction(func(L *lua.LState) int {
		names := syntax.ProviderNames()
		t := L.NewTable()
		for i, name := range names {
			t.RawSetInt(i+1, lua.LString(name))
		}
		L.Push(t)
		return 1
	}))

	return pkg
}

func providerFromTable(L *lua.LState, t *lua.LTable, owner string) (*syntax.Provider, error) {
	name := tableString(t, "name")
	if name == "" {
		return nil, errString("missing name")
	}
	request, ok := t.RawGetString("request").(*lua.LFunction)
	if !ok {
		return nil, errString("missing request function")
	}

	p := &syntax.Provider{
		Name:     namespaced(owner, name),
		Owner:    owner,
		Priority: int(tableNumber(t, "priority")),
	}
	p.Match.FileType = tableString(t, "filetype")

	var err error
	if p.Match.FileName, err = compileField(t, "filename"); err != nil {
		return nil, err
	}
	if p.Match.Header, err = compileField(t, "header"); err != nil {
		return nil, err
	}
	if p.Match.Signature, err = compileField(t, "signature"); err != nil {
		return nil, err
	}

	p.Request = func(req syntax.Request) {
		ctx := L.NewTable()
		L.SetField(ctx, "buf", luar.New(L, req.Buf))
		L.SetField(ctx, "id", lua.LNumber(req.BufID))
		L.SetField(ctx, "text", lua.LString(req.Text))
		L.SetField(ctx, "path", lua.LString(req.Path))
		L.SetField(ctx, "filetype", lua.LString(req.FileType))
		L.SetField(ctx, "first", lua.LNumber(req.First))
		L.SetField(ctx, "last", lua.LNumber(req.Last))
		L.SetField(ctx, "generation", lua.LNumber(req.Generation))
		L.SetField(ctx, "cause", lua.LString(causeName(req.Cause)))

		reply := L.NewFunction(func(L *lua.LState) int {
			req.Reply(parseFlatSpans(L, L.CheckTable(1)))
			return 0
		})

		if err := L.CallByParam(lua.P{Fn: request, NRet: 0, Protect: true}, ctx, reply); err != nil {
			if req.Fail != nil {
				req.Fail(err)
			}
			log.Println("syntax provider", p.Name, "failed:", err)
			if syntax.ReportError(p.Name) {
				screen.TermMessage("syntax: disabling provider " + p.Name + ": " + err.Error())
				buffer.RefreshSyntax()
			}
		}
	}

	return p, nil
}

type stringError string

func (e stringError) Error() string { return string(e) }

func errString(s string) error { return stringError(s) }

func namespaced(owner, name string) string {
	if owner == "" || strings.HasPrefix(name, owner+".") {
		return name
	}
	return owner + "." + name
}

func tableString(t *lua.LTable, key string) string {
	v := t.RawGetString(key)
	if v == lua.LNil {
		return ""
	}
	return lua.LVAsString(v)
}

func tableNumber(t *lua.LTable, key string) float64 {
	v := t.RawGetString(key)
	if v == lua.LNil {
		return 0
	}
	return float64(lua.LVAsNumber(v))
}

func compileField(t *lua.LTable, key string) (*regexp.Regexp, error) {
	expr := tableString(t, key)
	if expr == "" {
		return nil, nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, errString(key + ": " + err.Error())
	}
	return re, nil
}

func parseFlatSpans(L *lua.LState, t *lua.LTable) []syntax.Span {
	n := t.Len()
	spans := make([]syntax.Span, 0, n/4)
	for i := 1; i+3 <= n; i += 4 {
		line := int(lua.LVAsNumber(t.RawGetInt(i)))
		start := int(lua.LVAsNumber(t.RawGetInt(i + 1)))
		end := int(lua.LVAsNumber(t.RawGetInt(i + 2)))
		group := lua.LVAsString(t.RawGetInt(i + 3))
		spans = append(spans, syntax.Span{
			Line:      line,
			StartByte: start,
			EndByte:   end,
			Group:     syntax.Intern(group),
		})
	}
	return spans
}

func bufferFromLua(L *lua.LState, n int) *buffer.Buffer {
	if ud, ok := L.Get(n).(*lua.LUserData); ok {
		if b, ok := ud.Value.(*buffer.Buffer); ok {
			return b
		}
	}
	L.ArgError(n, "buffer expected")
	return nil
}

func hasColor(name string) bool {
	if name == "" {
		return false
	}
	cur := ""
	for i, part := range strings.Split(name, ".") {
		if i > 0 {
			cur += "."
		}
		cur += part
		if _, ok := config.Colorscheme[cur]; ok {
			return true
		}
	}
	return false
}

func causeName(c syntax.Cause) string {
	switch c {
	case syntax.CauseOpen:
		return "open"
	case syntax.CauseEdit:
		return "edit"
	case syntax.CauseManual:
		return "manual"
	case syntax.CauseReload:
		return "reload"
	default:
		return "unknown"
	}
}
