// Package inline implements render.Presenter for nib's default surface: the
// fzf-style inline widget that lives in the normal terminal scrollback (no alt
// screen, no mouse reporting). Its output is byte-for-byte what the hand-rolled
// rendering in tui/model.go produced before this package existed — this is a
// pure extraction, not a redesign.
package inline

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

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
	// OverlayDialogs is false: this surface lives in the normal scrollback, so
	// a dialog is content like any other — updateViewport appends it into the
	// same builder it feeds the viewport, and it scrolls away with history
	// exactly as every other message does. See full.Caps for the surface that
	// overlays instead.
	return render.Caps{AltScreen: false, Mouse: false, OverlayDialogs: false}
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

// contentPrefix returns the chrome this surface puts before a role's content:
// the label or gutter Message writes on the first line and indents the
// continuation lines to. Message and ContentWidth both read it, so the width
// the model pre-renders markdown at can never drift from the width this
// surface's chrome actually leaves — the model asks (ContentWidth) instead of
// reconstructing the prefix string itself.
func contentPrefix(role render.Role) string {
	switch role {
	case render.RoleUser:
		return theme.LabelYou.Render(theme.LabelYouText) + " " + theme.SepStyle.Render(theme.Sep) + " "
	case render.RoleAssistant:
		return theme.LabelNib.Render(theme.BrandName) + " " + theme.SepStyle.Render(theme.Sep) + " "
	case render.RoleAgent:
		return theme.Subtle.Render(theme.SubAgent) + " "
	case render.RoleTool:
		// A tool block's body is indented two cells beneath its own header line.
		return "  "
	case render.RoleError:
		return theme.Error.Render(theme.Cross) + " "
	}
	return ""
}

// ContentWidth reports how many cells are left for a role's content once this
// surface's chrome is accounted for. The model calls it to pre-render
// width-cached markdown (glamour) at the width this presenter will actually
// leave, instead of hard-coding one surface's prefix for both. The result is
// clamped to at least 1: a terminal narrower than the chrome must still give
// a renderer a legal width rather than zero or a negative one.
//
// This deliberately stays role-only (no prev parameter), even though
// messagePrefix below varies the prefix Message actually writes by prev: on a
// consecutive same-role run, messagePrefix swaps the label for an all-spaces
// prefix of the EXACT SAME width (see its doc), never a narrower or wider
// one. So the width contentPrefix reports is correct for every prev — the
// model never has to know which case it is, and pre-rendered markdown never
// needs re-wrapping when a message turns out to start a run instead of
// continuing one.
func (presenter) ContentWidth(role render.Role, w int) int {
	cw := w - lipgloss.Width(contentPrefix(role))
	if cw < 1 {
		cw = 1
	}
	return cw
}

// messagePrefix returns the prefix Message writes for role, given the
// previously rendered role. Repeating "you ·"/"nib ·" down a run of
// consecutive same-role messages costs six columns on every line and tells
// the reader nothing the blank-line separator didn't already say, so a
// consecutive user/assistant message gets an all-spaces prefix instead of the
// label — but at the SAME width as the label, never narrower. That is what
// keeps this in agreement with ContentWidth (which cannot see prev at all,
// see its doc) and keeps a run's content aligned down every line, not just
// its own: shrinking the prefix would shift content left the moment a run
// starts, and a Message call for a later line in the run never revisits an
// earlier one to re-align it.
//
// Only RoleUser/RoleAssistant branch on prev, matching the brief exactly:
// RoleAgent/RoleTool/RoleError keep their own unconditional chrome (the
// sub-agent marker, the tool body indent, the error cross) unchanged by this
// task.
func messagePrefix(role, prev render.Role) string {
	label := contentPrefix(role)
	if prev == role && (role == render.RoleUser || role == render.RoleAssistant) {
		return strings.Repeat(" ", lipgloss.Width(label))
	}
	return label
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
// prev is Phase 3 Task 12's first real consumer: on a run of consecutive
// same-role user/assistant messages, the label is dropped in favour of the
// blank-line separator (see messagePrefix) — the trailing-separator rule
// itself still never depends on prev; it was always "blank after every
// message, except a hugging agent line", and stays that way here.
func (presenter) Message(m render.Message, prev render.Role, w int) string {
	var body string
	switch m.Role {
	case render.RoleUser:
		prefix := messagePrefix(render.RoleUser, prev)
		wrapped := render.Wrap(m.Content, w-lipgloss.Width(prefix))
		body = prefixed(prefix, wrapped)

	case render.RoleAssistant:
		body = prefixed(messagePrefix(render.RoleAssistant, prev), m.Content)

	case render.RoleAgent:
		body = prefixed(contentPrefix(render.RoleAgent), styledLines(theme.Subtle, m.Content))
		if m.HugNext {
			return body
		}

	case render.RoleTool:
		// The label arrives already formatted (Message.Label): turning a tool
		// name and its raw JSON arguments into a human summary is domain logic
		// that stays model-side, same as markdown and the ask block.
		label := m.Label
		if m.AgentID != "" {
			label = theme.SubAgent + " " + render.ShortID(m.AgentID) + " · " + label
		}
		indent := contentPrefix(render.RoleTool)
		var b strings.Builder
		b.WriteString(theme.Subtle.Render(theme.Sep + " " + label))
		b.WriteString("\n")
		wrapped := render.Wrap(m.Content, w-lipgloss.Width(indent))
		for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
			b.WriteString(indent + theme.Help.Render(line))
			b.WriteString("\n")
		}
		body = b.String()

	case render.RoleError:
		prefix := contentPrefix(render.RoleError)
		wrapped := render.Wrap(m.Content, w-lipgloss.Width(prefix))
		body = prefixed(prefix, wrapped)

	default:
		return ""
	}
	return body + "\n"
}

// Reasoning renders the working indicator (spinner + status verb) and, when
// loading, the collapsible reasoning trace beneath it. Renders nothing when
// !v.Loading. The trace itself is capped through render.CollapsibleBox, which
// this task adds a producer for: v.Reasoning.Collapsed/MaxLines, populated by
// the model in viewState() from Model.reasoningCollapsed and
// theme.ReasoningMaxLines. Collapsed, the box shows the TRAILING lines of the
// trace (never the leading ones) — see CollapsibleBox's doc for why: a
// head-anchored box freezes on the trace's opening words and reads as a hang,
// while tailing doubles the box as its own progress indicator.
func (presenter) Reasoning(v render.ViewState, w int) string {
	if !v.Loading {
		return ""
	}
	var b strings.Builder
	b.WriteString(render.Loader(v.Spinner, v.Status, "", w))
	b.WriteString("\n")
	if strings.TrimSpace(v.Reasoning.Text) != "" {
		r := v.Reasoning
		box := render.CollapsibleBox{
			Lines:     strings.Split(strings.TrimRight(render.Wrap(r.Text, w-4), "\n"), "\n"),
			MaxLines:  r.MaxLines,
			Collapsed: r.Collapsed,
		}
		b.WriteString(theme.ReasoningHeader() + "\n")
		for _, line := range box.Visible() {
			b.WriteString("  " + theme.Subtle.Render(theme.BoxRule) + " " + theme.Reasoning.Render(line) + "\n")
		}
		// Default: expanded with nothing hidden — offer to collapse it back.
		// Collapsed with something hidden — offer to expand and say how much
		// is behind the fold. Collapsed with nothing hidden (a short trace
		// that never grew past MaxLines) — no hint at all: there is nothing
		// either affordance would change.
		hint := theme.ReasoningCollapse
		if n := box.Hidden(); n > 0 {
			hint = "… " + strconv.Itoa(n) + theme.ReasoningMore + theme.ReasoningExpand
		} else if r.Collapsed {
			hint = ""
		}
		if hint != "" {
			b.WriteString("  " + theme.Hint.Render(hint) + "\n")
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

// dialogAskMarker picks the leading marker for one ask_user option row: a
// checkbox when the dialog is multi-select (d.Checked non-nil), a radio
// otherwise, filled when checked/selected and hollow when not. i indexes
// d.Options; safe even if d.Checked is shorter (defensive, since a Presenter
// never constructs these values itself).
func dialogAskMarker(d render.Dialog, i int) string {
	if d.Checked != nil {
		if i < len(d.Checked) && d.Checked[i] {
			return theme.CheckOn
		}
		return theme.CheckOff
	}
	if i == d.Selected {
		return theme.RadioOn
	}
	return theme.RadioOff
}

// Dialog renders a modal prompt. DialogAsk lays out the question (Title),
// then one row per option — a cursor mark on the highlighted row, a radio or
// checkbox per dialogAskMarker, the row text in theme.ApproveKey when
// highlighted and theme.Help otherwise (mirroring DialogApproval's Emphasis
// styling) — and Hint beneath, wrapped. An ask with no options (free-text
// only) degrades to placing Title alone, same as before this task. DialogApproval
// lays out Rows (the argument card, or — when RowsUnstructured — a single
// wrapped prose block), Hint (the captured reasoning, wrapped) and Options
// (the choice menu, one line per option styled per its Emphasis).
func (presenter) Dialog(d render.Dialog, w int) string {
	switch d.Kind {
	case render.DialogAsk:
		gutter := theme.Gutter.Render(theme.ApprovalGutter) + " "
		var b strings.Builder
		b.WriteString(gutter + theme.LabelNib.Render(d.Title))
		b.WriteString("\n")
		for i, opt := range d.Options {
			cursor := "  "
			style := theme.Help
			if i == d.Selected {
				cursor = theme.Cursor + " "
				style = theme.ApproveKey
			}
			b.WriteString(gutter + cursor + dialogAskMarker(d, i) + " " + style.Render(opt.Text))
			b.WriteString("\n")
		}
		if d.Hint != "" {
			wrapped := render.Wrap(d.Hint, w-4)
			for _, line := range strings.Split(strings.TrimRight(wrapped, "\n"), "\n") {
				b.WriteString(gutter + theme.Hint.Render(line) + "\n")
			}
		}
		return b.String()

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
		case 5:
			// Classic approval menu: a blank gutter line, then the five options
			// (once / always / this turn / this session / deny-edit) — matching
			// the original hand-rolled block exactly.
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
func (p presenter) Footer(v render.ViewState, w int) string {
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

// FooterHeight reports how many terminal rows Footer occupies for this
// ViewState at this width — 1 for the bare help line, up to 7 once the
// new-output marker, an error line and the four job-status rows are all
// present. The shared core budgets the viewport against it, so an answer that
// disagrees with Footer by even one row makes the composed frame overflow the
// screen. Measuring the real output is the only way the two cannot drift as
// Phase 3 reshapes this surface's chrome.
func (p presenter) FooterHeight(v render.ViewState, w int) int {
	return lipgloss.Height(p.Footer(v, w))
}

// Frame composes the whole screen from its four already-rendered pieces. The
// inline widget lives in the normal scrollback, not the alt screen, so it has
// no screen of its own to lay these out on — it simply stacks them in the
// order they always rendered in: header, body, a blank line, the composer
// (completion popup / queue / textarea, whichever are present), a blank line,
// footer. w and h are unused here; this surface never had a frame to budget
// against before Task 10a, and does not gain one now — see full.Frame for the
// surface that does.
func (p presenter) Frame(v render.ViewState, header, body, composer, footer string, w, h int) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString(body)
	b.WriteString("\n")
	b.WriteString(composer)
	b.WriteString("\n")
	b.WriteString(footer)
	return b.String()
}
