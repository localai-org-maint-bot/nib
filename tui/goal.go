package tui

import (
	"fmt"

	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// goalFooterText builds the plain (unstyled, unglyphed) goal-footer text, or
// "" and false when no goal is set. Shared by renderGoalFooter (styled,
// directly unit-tested) and goalFooterRow (plain data for render.FooterRow).
func goalFooterText(goal string) (string, bool) {
	if goal == "" {
		return "", false
	}
	return fmt.Sprintf("goal: %s  (/goal clear)", render.TruncateRunes(goal, 48)), true
}

// renderGoalFooter renders a one-line indicator while a goal is active. Returns
// "" when no goal is set. Mirrors renderLoopsFooter's style.
func renderGoalFooter(goal string, width int) string {
	text, ok := goalFooterText(goal)
	if !ok {
		return ""
	}
	return theme.Subtle.Render(theme.Goal + " " + text)
}

// goalFooterRow returns the plain {Glyph, Text} data for the goal footer, and
// whether there is one to show. The presenter styles it.
func goalFooterRow(goal string) (render.FooterRow, bool) {
	text, ok := goalFooterText(goal)
	if !ok {
		return render.FooterRow{}, false
	}
	return render.FooterRow{Glyph: theme.Goal, Text: text}, true
}
