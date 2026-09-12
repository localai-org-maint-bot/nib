package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mudler/nib/theme"
)

// Loader composes the working indicator: spinner frame, status verb, and an
// optional right-docked trailer (e.g. an elapsed-time or job-count note).
// spinner is the already-chosen animation frame (theme.SpinnerFrames picks
// that; Loader does not reimplement frame selection). The trailer is dropped
// rather than allowed to overflow the row when width leaves less than a
// two-cell gap after the spinner and status.
func Loader(spinner, status, trailer string, width int) string {
	left := spinner
	if status != "" {
		if left != "" {
			left += " "
		}
		left += theme.Reasoning.Render(status)
	}
	if trailer == "" {
		return left
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(trailer)
	if gap < 2 {
		return left
	}
	return left + strings.Repeat(" ", gap) + theme.Meta.Render(trailer)
}
