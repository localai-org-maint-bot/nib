// Package full implements render.Presenter for nib's full-screen surface: the
// alt-screen mode with mouse reporting enabled. It starts as a byte-for-byte
// mirror of tui/render/inline, differing only in Caps() — landing the
// alt-screen wiring (tea.WithAltScreen/WithMouseCellMotion, suppressing the
// inline widget's region-clearing escapes) in isolation from the visual
// redesign this surface's chrome gets in Phase 3 (gutter instead of labels,
// framed dialogs).
package full

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// presenter is the full-screen Presenter. It holds no state: every method is
// a pure function of its arguments.
type presenter struct{}

// New returns the full-screen Presenter.
func New() render.Presenter {
	return presenter{}
}

func (presenter) Caps() render.Caps {
	return render.Caps{AltScreen: true, Mouse: true}
}

// prefixed lays out a block as `prefix + first line`, with continuation lines
// indented to the prefix width. This replaces the four near-identical loops the
// role switch used to carry.
func prefixed(prefix, content string) string {
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

// Message renders one chat entry, including the trailing blank-line separator
// before the next entry. assistant/agent content arrives already
// markdown-rendered (and wrapped) by the model — glamour is width-cached state
// the model owns, not something behind this interface. user/error content
// arrives raw and is wrapped here.
//
// Every role gets an unconditional trailing separator except RoleAgent with
// HugNext set: an agent lifecycle line whose thread run (rendered separately
// by the model, never through Message) immediately follows omits it, so the
// header visually hugs its own run instead of floating a blank line above it.
// That decision depends on the next RAW message (including agent_tool/
// agent_result, which Message never sees as a Message value), so the model
// computes HugNext and carries it on the value rather than Message re-deriving
// it from prev/Role.
//
// prev is accepted (not just for signature symmetry with the Presenter
// interface) so a different Presenter can vary spacing across role
// transitions; this implementation's spacing rule never depended on the
// previous role — it was always "blank after every message, except a hugging
// agent line" — so branching on prev here would be new behaviour, which a
// mirror of inline must not introduce.
func (presenter) Message(m render.Message, prev render.Role, w int) string {
	var body string
	switch m.Role {
	case render.RoleUser:
		prefix := theme.LabelYou.Render(theme.LabelYouText) + " " + theme.SepStyle.Render(theme.Sep) + " "
		wrapped := render.Wrap(m.Content, w-lipgloss.Width(prefix))
		body = prefixed(prefix, wrapped)

	case render.RoleAssistant:
		prefix := theme.LabelNib.Render(theme.BrandName) + " " + theme.SepStyle.Render(theme.Sep) + " "
		body = prefixed(prefix, m.Content)

	case render.RoleAgent:
		prefix := theme.Subtle.Render(theme.SubAgent) + " "
		body = prefixed(prefix, styledLines(theme.Subtle, m.Content))
		if m.HugNext {
			return body
		}

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
			label = theme.SubAgent + " " + render.ShortID(m.AgentID) + " · " + label
		}
		var b strings.Builder
		b.WriteString(theme.Subtle.Render(theme.Sep + " " + label))
		b.WriteString("\n")
		wrapped := render.Wrap(m.Content, w-2)
		for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
			b.WriteString("  " + theme.Help.Render(line))
			b.WriteString("\n")
		}
		body = b.String()

	case render.RoleError:
		prefix := theme.Error.Render(theme.Cross) + " "
		wrapped := render.Wrap(m.Content, w-lipgloss.Width(prefix))
		body = prefixed(prefix, wrapped)

	default:
		return ""
	}
	return body + "\n"
}

// Reasoning renders the working indicator (spinner + status verb) and, when
// loading, the collapsible reasoning trace beneath it. Renders nothing when
// !v.Loading.
func (presenter) Reasoning(v render.ViewState, w int) string {
	if !v.Loading {
		return ""
	}
	var b strings.Builder
	b.WriteString(v.Spinner + " " + theme.Reasoning.Render(v.Status))
	b.WriteString("\n")
	if v.Reasoning.Text != "" {
		b.WriteString(theme.ReasoningHeader() + "\n")
		wrapped := render.Wrap(v.Reasoning.Text, w-4)
		for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
			b.WriteString("  " + theme.Reasoning.Render(line) + "\n")
		}
	}
	return b.String()
}

// optionStyle picks the choice-menu style for one DialogOption: the bold
// actionable-key style when Emphasis is set, the dim hint style otherwise.
func optionStyle(o render.DialogOption) lipgloss.Style {
	if o.Emphasis {
		return theme.ApproveKey
	}
	return theme.Help
}

// Dialog renders a modal prompt. DialogAsk arrives with its whole block
// already composed in Title (the ask/multi-select block is domain logic that
// stays in tui, same precedent as markdown for Message) — Dialog here just
// places it. DialogApproval lays out Rows (the argument card, or — when
// RowsUnstructured — a single wrapped prose block), Hint (the captured
// reasoning, wrapped) and Options (the choice menu, one line per option styled
// per its Emphasis).
func (presenter) Dialog(d render.Dialog, w int) string {
	switch d.Kind {
	case render.DialogAsk:
		return d.Title + "\n"

	case render.DialogApproval:
		gutter := theme.Gutter.Render(theme.ApprovalGutter) + " "
		var b strings.Builder
		b.WriteString(gutter + theme.ApproveKey.Render(d.Title))
		b.WriteString("\n")

		if d.RowsUnstructured {
			// Fallback: unstructured args, wrapped and dimmed, no key column.
			if len(d.Rows) > 0 {
				wrapped := render.Wrap(d.Rows[0][1], w-4)
				for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
					b.WriteString(gutter + theme.Help.Render(line) + "\n")
				}
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
		case 0:
			// Nothing to render.
		case 1:
			// Edit-mode hint: single line, no leading blank.
			b.WriteString(gutter + optionStyle(d.Options[0]).Render(d.Options[0].Text))
			b.WriteString("\n")
		case 4:
			// Classic approval menu: a blank gutter line, then the four options —
			// matching the original hand-rolled block exactly.
			b.WriteString(gutter + "\n")
			for _, opt := range d.Options {
				b.WriteString(gutter + optionStyle(opt).Render(opt.Text) + "\n")
			}
		default:
			// Unexpected count: degrade visibly rather than render nothing,
			// still respecting each option's own Emphasis.
			for _, opt := range d.Options {
				b.WriteString(gutter + optionStyle(opt).Render(opt.Text) + "\n")
			}
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
	b.WriteString(left + strings.Repeat(" ", gap) + cwd)
	b.WriteString("\n")
	b.WriteString(theme.Hairline(v.Width))
	b.WriteString("\n")
	return b.String()
}

// footerRowStyle renders one FooterRow's text (glyph already prefixed), per
// its Kind. FooterJobs/FooterShell reproduce the original theme.Meta +
// width-fill treatment (Width doesn't just pad — lipgloss wraps content
// exceeding w, so a narrow terminal hard-wraps instead of spilling, exactly
// as before); everything else — FooterLoops/FooterGoal, and the zero-value
// FooterKindUnset (a forgotten Kind on some future fifth row) — gets the
// plain default: theme.Subtle, unfilled.
func footerRowStyle(kind render.FooterRowKind, text string, w int) string {
	switch kind {
	case render.FooterJobs, render.FooterShell:
		return theme.Meta.Width(w).Render(text)
	default:
		return theme.Subtle.Render(text)
	}
}

// Footer renders the new-output marker (when scrolled up with unread content
// below the fold), the help/badges line, the error line, and the job-status
// footer rows — everything that lives between the composer and the bottom of
// the screen.
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
	for _, row := range v.Footers {
		text := row.Text
		if row.Glyph != "" {
			text = row.Glyph + " " + text
		}
		b.WriteString("\n" + footerRowStyle(row.Kind, text, w))
	}
	return b.String()
}
