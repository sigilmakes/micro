package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	InitRuntimeFiles(false)
}

func TestAddFile(t *testing.T) {
	AddRuntimeFile(RTPlugin, memoryFile{"foo.lua", []byte("hello world\n")})

	f := FindRuntimeFile(RTPlugin, "foo.lua")
	assert.NotNil(t, f)
	assert.Equal(t, "foo.lua", f.Name())
	data, err := f.Data()
	assert.Nil(t, err)
	assert.Equal(t, []byte("hello world\n"), data)
}

func TestFindFile(t *testing.T) {
	f := FindRuntimeFile(RTHelp, "help")
	assert.NotNil(t, f)
	assert.Equal(t, "help", f.Name())
}

func TestRemovedSyntaxSlots(t *testing.T) {
	if FindRuntimeFile(1, "go") != nil {
		t.Fatal("syntax runtime slot should be empty")
	}
	if FindRuntimeFile(4, "go") != nil {
		t.Fatal("syntax header runtime slot should be empty")
	}
}
