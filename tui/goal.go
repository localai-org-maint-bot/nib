package tui

import (
	"fmt"

	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// renderGoalFooter renders a one-line indicator while a goal is active. Returns
// "" when no goal is set. Mirrors renderLoopsFooter's style.
func renderGoalFooter(goal string, width int) string {
	if goal == "" {
		return ""
	}
	line := fmt.Sprintf("%s goal: %s  (/goal clear)", theme.Goal, render.TruncateRunes(goal, 48))
	return theme.Subtle.Render(line)
}
