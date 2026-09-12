package tui

import (
	"fmt"
	"strings"

	wizmcp "github.com/mudler/nib/mcp"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// shellJobsFooterText builds the plain (unstyled, unglyphed) shell-jobs text,
// or "" when there are none. Shared by renderShellJobsFooter (styled, directly
// unit-tested) and shellJobsFooterRow (plain data for render.FooterRow).
func shellJobsFooterText(jobs []wizmcp.ShellJobInfo) string {
	if len(jobs) == 0 {
		return ""
	}
	var running, done, failed int
	for _, j := range jobs {
		switch j.Status {
		case "running":
			running++
		case "completed":
			done++
		case "failed":
			failed++
		}
	}
	parts := []string{fmt.Sprintf("shell: %d running", running)}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("%d done", done))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	parts = append(parts, "(ctrl+b background · ctrl+o logs)")
	return strings.Join(parts, "  ·  ")
}

// renderShellJobsFooter renders a compact one-line summary of shell jobs
// (background or backgrounded). Returns "" when there are none.
func renderShellJobsFooter(jobs []wizmcp.ShellJobInfo, width int) string {
	text := shellJobsFooterText(jobs)
	if text == "" {
		return ""
	}
	return jobsFooterStyle.Width(width).Render(theme.ShellJob + " " + text)
}

// shellJobsFooterRow returns the plain {Glyph, Text} data for the shell-jobs
// footer, and whether there is one to show. The presenter styles it.
func shellJobsFooterRow(jobs []wizmcp.ShellJobInfo) (render.FooterRow, bool) {
	text := shellJobsFooterText(jobs)
	if text == "" {
		return render.FooterRow{}, false
	}
	return render.FooterRow{Glyph: theme.ShellJob, Text: text}, true
}
