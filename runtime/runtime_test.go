package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssetDir(t *testing.T) {
	t.Parallel()
	// Test AssetDir
	entries, err := AssetDir("colorschemes")
	assert.NoError(t, err)
	assert.Contains(t, entries, "default.micro")
	assert.True(t, len(entries) > 5)
}
