package tui

import (
	"os"
	"testing"
)

// TestMain points the default config root at a throwaway directory, so a
// session built with an empty BaseDir never reads the developer's real
// credentials.json or the provider saved by /login (provider.json).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nib-tui-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
