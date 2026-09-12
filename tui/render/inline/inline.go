// Package inline implements render.Presenter for nib's default surface: the
// fzf-style inline widget that lives in the normal terminal scrollback (no alt
// screen, no mouse reporting). Its output is byte-for-byte what the hand-rolled
// rendering in tui/model.go produced before this package existed — this is a
// pure extraction, not a redesign.
package inline

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// presenter is the inline-widget Presenter. It holds no state: every method is
// a pure function of its arguments.
type presenter struct{}

// New returns the inline Presenter.
func New() render.Presenter {
	return presenter{}
}

func (presenter) Caps() render.Caps {
	return render.Caps{AltScreen: false, Mouse: false}
}

// prefixed lays out a block as `prefix + first line`, with continuation lines
// indented to the prefix width. This replaces the four near-identical loops the
// role switch used to carry.
func prefixed(prefix, content string, width int) string {
	pw := lipgloss.Width(prefix)
	var b strings.Builder
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for i, line := range lines {
		if i == 0 {
			b.WriteString(prefix)
		} else {
			b.WriteString(strings.Repeat(" ", pw))
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// styledLines applies style.Render to each line of content independently
// (rather than to the joined block), matching the original per-line styling
// the hand-rolled agent-message loop used to do.
func styledLines(style lipgloss.Style, content string) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for i, line := range lines {
		lines[i] = style.Render(line)
	}
	return strings.Join(lines, "\n")
}

// shortID truncates an id to a compact display form. Duplicated from
// tui.shortID (tui/agents.go) rather than exported across the package
// boundary: it is a trivial, self-contained one-liner and the presenter must
// not import the tui package (tui imports render; the reverse would cycle).
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Message renders one chat entry. assistant/agent content arrives already
// markdown-rendered (and wrapped) by the model — glamour is width-cached state
// the model owns, not something behind this interface. user/error content
// arrives raw and is wrapped here.
func (presenter) Message(m render.Message, prev render.Role, w int) string {
	switch m.Role {
	case render.RoleUser:
		prefix := theme.LabelYou.Render("you") + " " + theme.SepStyle.Render(theme.Sep) + " "
		wrapped := render.Wrap(m.Content, w-lipgloss.Width(prefix))
		return prefixed(prefix, wrapped, w)

	case render.RoleAssistant:
		prefix := theme.LabelNib.Render(theme.BrandName) + " " + theme.SepStyle.Render(theme.Sep) + " "
		return prefixed(prefix, m.Content, w)

	case render.RoleAgent:
		prefix := theme.Subtle.Render(theme.SubAgent) + " "
		return prefixed(prefix, styledLines(theme.Subtle, m.Content), w)

	case render.RoleTool:
		label := m.Name
		if m.Arguments != "" {
			// First line of the friendly summary makes the clearest header.
			summary := chat.FormatToolCall(m.Name, m.Arguments)
			if nl := strings.IndexByte(summary, '\n'); nl >= 0 {
				summary = summary[:nl]
			}
			if summary != "" {
				label = summary
			}
		}
		if m.AgentID != "" {
			label = theme.SubAgent + " " + shortID(m.AgentID) + " · " + label
		}
		var b strings.Builder
		b.WriteString(theme.Subtle.Render(theme.Sep + " " + label))
		b.WriteString("\n")
		wrapped := render.Wrap(m.Content, w-2)
		for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
			b.WriteString("  " + theme.Help.Render(line))
			b.WriteString("\n")
		}
		return b.String()

	case render.RoleError:
		prefix := theme.Error.Render(theme.Cross) + " "
		wrapped := render.Wrap(m.Content, w-lipgloss.Width(prefix))
		return prefixed(prefix, wrapped, w)
	}
	return ""
}

// Reasoning renders the working indicator (spinner + status verb) and, when
// present, the collapsible reasoning trace beneath it.
func (presenter) Reasoning(r render.Reasoning, w int) string {
	var b strings.Builder
	b.WriteString(r.Spinner + " " + theme.Reasoning.Render(r.Status))
	b.WriteString("\n")
	if r.Text != "" {
		b.WriteString(theme.ReasoningHeader() + "\n")
		wrapped := render.Wrap(r.Text, w-4)
		for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
			b.WriteString("  " + theme.Reasoning.Render(line) + "\n")
		}
	}
	return b.String()
}

// Dialog renders a modal prompt. DialogAsk arrives with its whole block
// already composed in Title (the ask/multi-select block is domain logic that
// stays in tui, same precedent as markdown for Message) — Dialog here just
// places it. DialogApproval is decomposed into Rows (the argument card; a
// single row with an empty key is the fallback unstructured-args block) and
// Options (either the 4-line choice menu, styled per the fixed convention
// below, or the single edit-mode hint).
func (presenter) Dialog(d render.Dialog, w int) string {
	switch d.Kind {
	case render.DialogAsk:
		return d.Title + "\n"

	case render.DialogApproval:
		gutter := theme.Gutter.Render(theme.ApprovalGutter) + " "
		var b strings.Builder
		b.WriteString(gutter + theme.ApproveKey.Render(d.Title))
		b.WriteString("\n")

		if len(d.Rows) == 1 && d.Rows[0][0] == "" {
			// Fallback: unstructured args, wrapped and dimmed, no key column.
			wrapped := render.Wrap(d.Rows[0][1], w-4)
			for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
				b.WriteString(gutter + theme.Help.Render(line) + "\n")
			}
		} else if len(d.Rows) > 0 {
			maxKey := 0
			for _, row := range d.Rows {
				if len(row[0]) > maxKey {
					maxKey = len(row[0])
				}
			}
			for _, row := range d.Rows {
				key := row[0] + strings.Repeat(" ", maxKey-len(row[0]))
				val := render.TruncateLine(row[1], w-8-maxKey)
				b.WriteString(gutter + "  " + theme.Meta.Render(key) + "  " + theme.Help.Render(val) + "\n")
			}
		}

		if d.Hint != "" {
			wrapped := render.Wrap(d.Hint, w-4)
			for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
				b.WriteString(gutter + theme.Reasoning.Render(line) + "\n")
			}
		}

		switch len(d.Options) {
		case 1:
			// Edit-mode hint: single line, same key styling as the choice menu.
			b.WriteString(gutter + theme.ApproveKey.Render(d.Options[0]))
			b.WriteString("\n")
		case 4:
			// Choice menu: once / always / this-turn are actionable keys; the
			// deny-edit line is a dimmer, non-key hint — matching the original
			// hand-rolled block exactly.
			b.WriteString(gutter + "\n")
			b.WriteString(gutter + theme.ApproveKey.Render(d.Options[0]) + "\n")
			b.WriteString(gutter + theme.ApproveKey.Render(d.Options[1]) + "\n")
			b.WriteString(gutter + theme.ApproveKey.Render(d.Options[2]) + "\n")
			b.WriteString(gutter + theme.Help.Render(d.Options[3]) + "\n")
		}
		return b.String()
	}
	return ""
}

// Header renders the brand/badge line and the rule beneath it.
func (presenter) Header(v render.ViewState) string {
	var b strings.Builder
	left := theme.Brand.Render(v.Brand)
	if v.AutoApprove {
		left += "  " + theme.Yolo.Render(theme.YoloBadge)
	}
	cwd := theme.Meta.Render(v.Cwd)
	gap := v.Width - lipgloss.Width(left) - lipgloss.Width(cwd)
	if gap < 1 {
		gap = 1
	}
	width := v.Width
	if width < 1 {
		width = 1
	}
	b.WriteString(left + strings.Repeat(" ", gap) + cwd)
	b.WriteString("\n")
	b.WriteString(theme.Rule.Render(strings.Repeat("─", width)))
	b.WriteString("\n")
	return b.String()
}

// Footer renders the new-output marker (when scrolled up with unread content
// below the fold), the help/badges line, the error line, and the job footers —
// everything that lives between the composer and the bottom of the screen.
func (presenter) Footer(v render.ViewState, w int) string {
	var b strings.Builder
	if v.NewOutput {
		b.WriteString(theme.NewOutputMarker())
		b.WriteString("\n")
	}
	if v.Badges != "" {
		gap := w - lipgloss.Width(v.Help) - lipgloss.Width(v.Badges)
		if gap < 1 {
			gap = 1
		}
		b.WriteString(v.Help + strings.Repeat(" ", gap) + v.Badges)
	} else {
		b.WriteString(v.Help)
	}
	if v.Err != "" {
		b.WriteString("\n" + theme.Error.Render(theme.Cross+" "+v.Err))
	}
	for _, f := range []string{v.JobsFooter, v.ShellJobsFooter, v.LoopsFooter, v.GoalFooter} {
		if f != "" {
			b.WriteString("\n" + f)
		}
	}
	return b.String()
}
