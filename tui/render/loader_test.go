package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestLoaderComposesSpinnerAndStatus(t *testing.T) {
	got := Loader("-", "thinking", "", 40)
	if !strings.Contains(got, "-") || !strings.Contains(got, "thinking") {
		t.Errorf("Loader() = %q, want it to contain spinner and status", got)
	}
}

func TestLoaderDocksTrailerRight(t *testing.T) {
	got := Loader("-", "working", "3 jobs", 30)
	if !strings.HasSuffix(got, "3 jobs") {
		t.Errorf("Loader() = %q, want it to end with the trailer", got)
	}
	if lipgloss.Width(got) > 30 {
		t.Errorf("Loader() width = %d, want <= 30", lipgloss.Width(got))
	}
}

// TestLoaderDropsTrailerWhenTooNarrow pins the brief's rule: drop the
// trailer rather than overflow when the row leaves less than a two-cell gap.
func TestLoaderDropsTrailerWhenTooNarrow(t *testing.T) {
	spinner, status, trailer := "-", "working", "3 jobs"
	left := Loader(spinner, status, "", 100)
	width := lipgloss.Width(left) // exactly no room left for a trailer
	got := Loader(spinner, status, trailer, width)
	if strings.Contains(got, trailer) {
		t.Errorf("Loader() = %q, want the trailer dropped at width %d", got, width)
	}
	if got != left {
		t.Errorf("Loader() = %q, want just the left side %q", got, left)
	}
}

func TestLoaderNoTrailer(t *testing.T) {
	got := Loader("-", "working", "", 10)
	if strings.TrimSpace(got) != got {
		// no trailing padding should be appended when there is no trailer
		t.Errorf("Loader() = %q, want no trailing whitespace", got)
	}
}
