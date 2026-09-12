package render

import (
	"strings"
	"testing"
)

func TestLoaderComposesSpinnerAndStatus(t *testing.T) {
	got := Loader("-", "thinking")
	if !strings.Contains(got, "-") || !strings.Contains(got, "thinking") {
		t.Errorf("Loader() = %q, want it to contain spinner and status", got)
	}
}

func TestLoaderNoTrailingWhitespace(t *testing.T) {
	got := Loader("-", "working")
	if strings.TrimSpace(got) != got {
		t.Errorf("Loader() = %q, want no trailing whitespace", got)
	}
}
