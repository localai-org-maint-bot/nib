// Package full implements render.Presenter for nib's full-screen surface: the
// alt-screen mode with mouse reporting enabled. It started as a byte-for-byte
// mirror of tui/render/inline, differing only in Caps() — landing the
// alt-screen wiring (tea.WithAltScreen/WithMouseCellMotion, suppressing the
// inline widget's region-clearing escapes) in isolation from the visual
// redesign this surface's chrome gets in Phase 3. Task 12 is the first cut of
// that redesign: RoleUser/RoleAssistant now get a coloured gutter instead of
// inline's word label (see contentPrefix and gutterLines); everything else —
// Reasoning, Dialog, Header, Footer, and the RoleAgent/RoleTool/RoleError
// chrome — still mirrors inline exactly, and the conformance suite
// (tui/render/conformance_test.go) is what proves the divergence stops there.
package full

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

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
	// OverlayDialogs is true: this surface owns the whole alt screen, so Frame
	// places v.Dialogs itself, freshly, every frame — not baked into body as
	// scrollback content that would scroll away with history (and, since the
	// core still builds body from the same viewport machinery inline uses,
	// would otherwise render twice). See inline.Caps for the surface that
	// stacks instead.
	return render.Caps{AltScreen: true, Mouse: true, OverlayDialogs: true}
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
// the gutter or label Message writes, and the width it reserves on every
// line. Message and ContentWidth both read it, so the width the model
// pre-renders markdown at can never drift from the width this surface's
// chrome actually leaves — the model asks (ContentWidth) instead of
// reconstructing the prefix string itself.
//
// RoleUser/RoleAssistant get a one-column coloured gutter (theme.MsgGutter)
// instead of inline's word label ("you ·"/"nib ·"): this surface owns the
// whole alt screen, so a colour down the left edge of every line of the block
// (see gutterLines) identifies the speaker without spending a word on it,
// where inline's tighter columns keep the word but drop it on a run (Phase 3
// Task 12). RoleAgent/RoleTool/RoleError are unchanged by this task — their
// own chrome (the sub-agent marker, the tool body indent, the error cross)
// stays a label, matching inline.
func contentPrefix(role render.Role) string {
	switch role {
	case render.RoleUser:
		return theme.LabelYou.Render(theme.MsgGutter) + " "
	case render.RoleAssistant:
		return theme.Gutter.Render(theme.MsgGutter) + " "
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

// gutterLines lays out a block with the gutter prefix repeated on every
// non-blank line, rather than only on the first line the way prefixed's label
// layout does — a gutter's whole point is to mark the block as it scrolls by,
// not to head it once. A blank line inside the content (a markdown paragraph
// break) is left bare: painting the bar across it would read as the message
// continuing through the gap rather than pausing for one, and would silently
// merge what must stay two visually separate paragraphs (and, cross-surface,
// two separate blocks — see TestMessageStructuralEquivalence's "multiline
// content" case, which requires both presenters to agree on block count).
func gutterLines(prefix, content string) string {
	var b strings.Builder
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			b.WriteString(prefix)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// ContentWidth reports how many cells are left for a role's content once this
// surface's chrome is accounted for. The model calls it to pre-render
// width-cached markdown (glamour) at the width this presenter will actually
// leave, instead of hard-coding one surface's prefix for both. The result is
// clamped to at least 1: a terminal narrower than the chrome must still give
// a renderer a legal width rather than zero or a negative one.
func (presenter) ContentWidth(role render.Role, w int) int {
	cw := w - lipgloss.Width(contentPrefix(role))
	if cw < 1 {
		cw = 1
	}
	return cw
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
// prev is accepted for signature symmetry with the Presenter interface, but
// this surface's chrome never reads it: the gutter (see contentPrefix) marks
// every message regardless of what rendered before it, since a one-column
// colour bar costs nothing to repeat the way a word label does — inline is
// the surface with columns tight enough to make that trade (Phase 3 Task 12).
// The trailing-separator rule likewise never depended on the previous role —
// it was always "blank after every message, except a hugging agent line".
func (presenter) Message(m render.Message, prev render.Role, w int) string {
	var body string
	switch m.Role {
	case render.RoleUser:
		prefix := contentPrefix(render.RoleUser)
		wrapped := render.Wrap(m.Content, w-lipgloss.Width(prefix))
		body = gutterLines(prefix, wrapped)

	case render.RoleAssistant:
		body = gutterLines(contentPrefix(render.RoleAssistant), m.Content)

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
// while tailing doubles the box as its own progress indicator. This mirrors
// inline byte-for-byte, same precedent as the rest of this file — the
// conformance suite enforces it.
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
// only) degrades to placing Title alone, same as before this task. This
// mirrors inline byte-for-byte, same precedent as the rest of this file — the
// conformance suite enforces it. DialogApproval lays out Rows (the argument
// card, or — when RowsUnstructured — a single wrapped prose block), Hint (the
// captured reasoning, wrapped) and Options (the choice menu, one line per
// option styled per its Emphasis).
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

// Frame composes the whole screen from its four already-rendered pieces:
// header, body, then every pending v.Dialogs overlay, then composer
// (completion popup / queue / textarea, whichever are present), then footer —
// header/body/composer/footer stacked in the same order the core always
// concatenated them in, with dialogs docked directly above the composer
// (Phase 3 Task 11).
//
// Docked above the composer, not centred over body: this is where the
// tool-approval block has always visually sat on the inline surface (the last
// thing updateViewport pushes before the composer), so keeping it there on
// this surface too means the two never disagree about WHERE a pending
// question appears, only about HOW it gets there. A centred floating box
// would need real (row, col) placement — lipgloss.Place or manual line
// splicing over body's own lines — machinery nothing else in either presenter
// uses, for a payoff (visual centring) this phase's design doesn't call for.
//
// "Overlay" names what changed, not where: OverlayDialogs (see Caps) means
// Frame places v.Dialogs itself, fresh every frame from the ViewState, rather
// than the core having baked them into body's scrollback (inline's approach —
// see its Caps and updateViewport in tui/model.go). That is what stops a
// dialog from scrolling away with history when the user scrolls the
// transcript, and from rendering twice now that the core skips its own
// scrollback append for a surface that declares OverlayDialogs.
func (p presenter) Frame(v render.ViewState, header, body, composer, footer string, w, h int) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString(body)
	b.WriteString("\n")
	for _, d := range v.Dialogs {
		b.WriteString(p.Dialog(d, w))
	}
	b.WriteString(composer)
	b.WriteString("\n")
	b.WriteString(footer)
	return b.String()
}
