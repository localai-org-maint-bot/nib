package render_test

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
	"github.com/mudler/nib/tui/render/full"
	"github.com/mudler/nib/tui/render/inline"
)

// presenters is every implementation. Adding one here is how a new surface
// proves it behaves like the others.
func presenters() map[string]render.Presenter {
	return map[string]render.Presenter{
		"inline": inline.New(),
		"full":   full.New(),
	}
}

// presenterNames returns the presenter keys in a stable order, so the first is
// always the same reference every other implementation is compared against and
// a failure names a deterministic pair.
func presenterNames() []string {
	var names []string
	for name := range presenters() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes SGR escape sequences so width/content checks measure only
// visible runes.
func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// The conformance suite below enforces the phase's one structural rule: the two
// surfaces diverge in CHROME but never in LOGIC. `full` is currently a copy of
// `inline` and Phase 3 deliberately pulls their chrome apart (a gutter instead
// of labels, framed dialogs), so a shared base implementation would have to be
// re-cut immediately. These tests are the guard that replaces it: they compare
// the two presenters' STRUCTURE — how many blocks, in what order, with the
// input's content in the same relative positions, the same trailing-separator
// decision, the same option count — while staying blind to the styling,
// glyphs, prefixes and extra frame rows that are each surface's own business.
//
// A structural fingerprint is deliberately made of things chrome cannot
// change:
//
//   - blocks: how many blank-line-separated groups the output has. Restyling a
//     line or prefixing it with a gutter does not regroup the output; dropping
//     or merging a block does.
//   - trailingSep: whether the chunk ends in the blank-line separator before
//     the next one. This is the HugNext rule, which is logic, not decoration.
//   - sequence: the order the input's own substrings come back in, and which of
//     them share a line ("|" same line, "/" a later line). Immune to prefixes
//     and to extra frame rows, sensitive to content being dropped, reordered,
//     or collapsed onto one line.

// fingerprint is the chrome-blind structural signature of one rendered chunk.
type fingerprint struct {
	Blocks      int
	TrailingSep bool
	Sequence    string
}

// blocks counts the blank-line-separated groups in a rendered chunk. A "blank"
// line is one with no visible content at all, so a gutter-only spacer row (the
// approval menu's leading `▏` line) correctly does NOT split a block — it is
// chrome inside one.
func blocks(out string) int {
	n := 0
	inBlock := false
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(stripANSI(line)) == "" {
			inBlock = false
			continue
		}
		if !inBlock {
			n++
			inBlock = true
		}
	}
	return n
}

// sequence reports the order tokens appear in the output and which of them
// share a line: tokens on one line are joined with "|", a move to a later line
// is "/". Tokens the output dropped are omitted here and reported separately by
// assertPreserves, so a drop shows up as both a missing substring and a
// structural difference.
func sequence(out string, tokens []string) string {
	type hit struct {
		line, col int
		token     string
	}
	var hits []hit
	lines := strings.Split(stripANSI(out), "\n")
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		for i, line := range lines {
			if col := strings.Index(line, tok); col >= 0 {
				hits = append(hits, hit{line: i, col: col, token: tok})
				break
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].line != hits[j].line {
			return hits[i].line < hits[j].line
		}
		return hits[i].col < hits[j].col
	})
	var b strings.Builder
	for i, h := range hits {
		if i > 0 {
			if h.line == hits[i-1].line {
				b.WriteString("|")
			} else {
				b.WriteString("/")
			}
		}
		b.WriteString(h.token)
	}
	return b.String()
}

func fingerprintOf(out string, tokens []string) fingerprint {
	return fingerprint{
		Blocks:      blocks(out),
		TrailingSep: strings.HasSuffix(out, "\n\n"),
		Sequence:    sequence(out, tokens),
	}
}

// assertPreserves fails when the rendered output dropped any of the input's own
// text. Chrome is a presenter's business; the user's words are not.
func assertPreserves(t *testing.T, name, out string, tokens []string) {
	t.Helper()
	plain := stripANSI(out)
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		if !strings.Contains(plain, tok) {
			t.Errorf("%s dropped %q from its output: %q", name, tok, out)
		}
	}
}

// assertFitsWidth fails when any line exceeds the budget, which would corrupt
// the surrounding shell on the inline widget and wrap unpredictably on the
// alt screen.
func assertFitsWidth(t *testing.T, name, out string, w int) {
	t.Helper()
	for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if got := len([]rune(stripANSI(line))); got > w {
			t.Errorf("%s line %d is %d cells wide, budget %d: %q", name, i, got, w, line)
		}
	}
}

// assertSameFingerprint compares every presenter against the first one by
// name. Differing fingerprints mean the surfaces disagree about something
// other than chrome.
func assertSameFingerprint(t *testing.T, fps map[string]fingerprint) {
	t.Helper()
	names := presenterNames()
	ref := names[0]
	for _, name := range names[1:] {
		if fps[name] != fps[ref] {
			t.Errorf("structural drift between %s and %s:\n %s = %+v\n %s = %+v",
				ref, name, ref, fps[ref], name, fps[name])
		}
	}
}

// TestAllPresentersRenderMessageContent: chrome may differ, the user's words may not.
func TestAllPresentersRenderMessageContent(t *testing.T) {
	for name, p := range presenters() {
		t.Run(name, func(t *testing.T) {
			out := p.Message(render.Message{Role: render.RoleUser, Content: "distinctive content"}, render.RoleNone, 80)
			if !strings.Contains(out, "distinctive content") {
				t.Errorf("%s dropped the message content: %q", name, out)
			}
		})
	}
}

// TestAllPresentersRespectWidth: no presenter may emit a line wider than the
// budget it was given, or the inline widget corrupts the surrounding shell.
func TestAllPresentersRespectWidth(t *testing.T) {
	const w = 40
	for name, p := range presenters() {
		t.Run(name, func(t *testing.T) {
			// RoleUser deliberately: Message wraps user content itself. Assistant
			// and agent content arrives pre-rendered (glamour is width-cached
			// state owned by the model), so it is not the presenter's to wrap.
			out := p.Message(render.Message{
				Role:    render.RoleUser,
				Content: strings.Repeat("overflowing ", 30),
			}, render.RoleNone, w)
			for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
				if got := len([]rune(stripANSI(line))); got > w {
					t.Errorf("%s line %d is %d cells wide, budget %d: %q", name, i, got, w, line)
				}
			}
		})
	}
}

// TestMessageStructuralEquivalence walks every Role (and the flags that change
// how one is laid out) and requires all presenters to agree structurally:
// same blocks, same content order, same trailing-separator decision. wantSep is
// stated per case rather than merely compared, so the two surfaces drifting
// TOGETHER away from the HugNext rule is a failure too.
func TestMessageStructuralEquivalence(t *testing.T) {
	const w = 60
	cases := []struct {
		name    string
		msg     render.Message
		prev    render.Role
		tokens  []string
		wantSep bool
		// wraps marks the roles whose content the presenter itself wraps;
		// pre-rendered (glamour) content is not the presenter's to fit.
		wraps bool
	}{
		{
			name:    "user",
			msg:     render.Message{Role: render.RoleUser, Content: "user says this"},
			tokens:  []string{"user says this"},
			wantSep: true,
			wraps:   true,
		},
		{
			name:    "user after assistant",
			msg:     render.Message{Role: render.RoleUser, Content: "second turn"},
			prev:    render.RoleAssistant,
			tokens:  []string{"second turn"},
			wantSep: true,
			wraps:   true,
		},
		{
			name:    "assistant",
			msg:     render.Message{Role: render.RoleAssistant, Content: "assistant answer"},
			tokens:  []string{"assistant answer"},
			wantSep: true,
		},
		{
			name:    "agent",
			msg:     render.Message{Role: render.RoleAgent, Content: "agent started", AgentID: "abcdef123456"},
			tokens:  []string{"agent started"},
			wantSep: true,
		},
		{
			// The HugNext rule: the thread run that follows hugs this line, so
			// the separator must be omitted. This is logic, not decoration —
			// both surfaces owe the same answer.
			name:    "agent hugging its thread run",
			msg:     render.Message{Role: render.RoleAgent, Content: "agent started", AgentID: "abcdef123456", HugNext: true},
			tokens:  []string{"agent started"},
			wantSep: false,
		},
		{
			name:    "tool",
			msg:     render.Message{Role: render.RoleTool, Label: "read file.go", Content: "tool output body"},
			tokens:  []string{"read file.go", "tool output body"},
			wantSep: true,
			wraps:   true,
		},
		{
			name:    "tool from a sub-agent",
			msg:     render.Message{Role: render.RoleTool, Label: "read file.go", Content: "tool output body", AgentID: "abcdef123456"},
			tokens:  []string{"read file.go", "tool output body"},
			wantSep: true,
			wraps:   true,
		},
		{
			name:    "error",
			msg:     render.Message{Role: render.RoleError, Content: "it went wrong"},
			tokens:  []string{"it went wrong"},
			wantSep: true,
			wraps:   true,
		},
		{
			name:   "multiline content",
			msg:    render.Message{Role: render.RoleAssistant, Content: "first para\n\nsecond para"},
			tokens: []string{"first para", "second para"},
			// Two paragraphs must stay two blocks and stay in order; the
			// fingerprint comparison is what enforces that across surfaces.
			wantSep: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fps := map[string]fingerprint{}
			for name, p := range presenters() {
				out := p.Message(tc.msg, tc.prev, w)
				assertPreserves(t, name, out, tc.tokens)
				if tc.wraps {
					assertFitsWidth(t, name, out, w)
				}
				if got := strings.HasSuffix(out, "\n\n"); got != tc.wantSep {
					t.Errorf("%s trailing separator = %v, want %v: %q", name, got, tc.wantSep, out)
				}
				fps[name] = fingerprintOf(out, tc.tokens)
			}
			assertSameFingerprint(t, fps)
		})
	}
}

// TestUnknownRoleRendersNothing pins the shared contract for a Role no
// presenter knows: render nothing at all rather than an unlabelled body.
func TestUnknownRoleRendersNothing(t *testing.T) {
	for name, p := range presenters() {
		if out := p.Message(render.Message{Role: render.RoleNone, Content: "orphan"}, render.RoleNone, 60); out != "" {
			t.Errorf("%s rendered %q for an unknown role, want empty", name, out)
		}
	}
}

// TestDialogStructuralEquivalence covers every DialogKind and every branch
// Dialog has: the pre-rendered ask block, the approval card with structured
// rows, the RowsUnstructured prose fallback, and each of the option counts the
// type documents (0, 1, the classic 4, and an unexpected count that must still
// degrade visibly rather than vanish).
func TestDialogStructuralEquivalence(t *testing.T) {
	const w = 60
	approvalRows := [][2]string{{"path", "main.go"}, {"mode", "0644"}}

	cases := []struct {
		name   string
		dialog render.Dialog
		tokens []string
		// options is what every presenter must render one line for.
		options []string
	}{
		{
			name: "ask block",
			dialog: render.Dialog{
				Kind:  render.DialogAsk,
				Title: "which one?\n  1. alpha\n  2. beta",
			},
			tokens: []string{"which one?", "1. alpha", "2. beta"},
		},
		{
			name: "approval with four options",
			dialog: render.Dialog{
				Kind:    render.DialogApproval,
				Title:   "write main.go",
				Rows:    approvalRows,
				Hint:    "needs the fix applied",
				Options: []render.DialogOption{{Text: "[y] yes", Emphasis: true}, {Text: "[a] always", Emphasis: true}, {Text: "[t] turn", Emphasis: true}, {Text: "[n] no", Emphasis: false}},
			},
			tokens:  []string{"write main.go", "path", "main.go", "mode", "0644", "needs the fix applied", "[y] yes", "[a] always", "[t] turn", "[n] no"},
			options: []string{"[y] yes", "[a] always", "[t] turn", "[n] no"},
		},
		{
			name: "approval in edit mode",
			dialog: render.Dialog{
				Kind:    render.DialogApproval,
				Title:   "write main.go",
				Rows:    approvalRows,
				Options: []render.DialogOption{{Text: "enter to send", Emphasis: true}},
			},
			tokens:  []string{"write main.go", "path", "mode", "enter to send"},
			options: []string{"enter to send"},
		},
		{
			name: "approval with unstructured arguments",
			dialog: render.Dialog{
				Kind:             render.DialogApproval,
				Title:            "run something",
				Rows:             [][2]string{{"", "a raw prose block describing the call"}},
				RowsUnstructured: true,
				Options:          []render.DialogOption{{Text: "[y] yes", Emphasis: true}},
			},
			tokens:  []string{"run something", "a raw prose block describing the call", "[y] yes"},
			options: []string{"[y] yes"},
		},
		{
			name: "approval with no options",
			dialog: render.Dialog{
				Kind:  render.DialogApproval,
				Title: "write main.go",
				Rows:  approvalRows,
			},
			tokens: []string{"write main.go", "path", "mode"},
		},
		{
			name: "approval with an unexpected option count",
			dialog: render.Dialog{
				Kind:    render.DialogApproval,
				Title:   "write main.go",
				Options: []render.DialogOption{{Text: "[y] yes", Emphasis: true}, {Text: "[n] no", Emphasis: false}},
			},
			tokens:  []string{"write main.go", "[y] yes", "[n] no"},
			options: []string{"[y] yes", "[n] no"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fps := map[string]fingerprint{}
			for name, p := range presenters() {
				out := p.Dialog(tc.dialog, w)
				assertPreserves(t, name, out, tc.tokens)
				// Every option gets its own line: collapsing two onto one, or
				// silently dropping one, changes what the user can press.
				if got := optionLines(out, tc.options); got != len(tc.options) {
					t.Errorf("%s rendered %d option lines, want %d: %q", name, got, len(tc.options), out)
				}
				fps[name] = fingerprintOf(out, tc.tokens)
			}
			assertSameFingerprint(t, fps)
		})
	}
}

// optionLines counts the distinct output lines carrying one of the option
// texts, so two options sharing a line counts as one.
func optionLines(out string, options []string) int {
	if len(options) == 0 {
		return 0
	}
	seen := map[int]bool{}
	lines := strings.Split(stripANSI(out), "\n")
	for i, line := range lines {
		for _, opt := range options {
			if strings.Contains(line, opt) {
				seen[i] = true
			}
		}
	}
	return len(seen)
}

// TestUnknownDialogKindRendersNothing: a kind no presenter handles must produce
// nothing on every surface, not a half-drawn card on one of them.
func TestUnknownDialogKindRendersNothing(t *testing.T) {
	for name, p := range presenters() {
		if out := p.Dialog(render.Dialog{Kind: render.DialogResume, Title: "resume?"}, 60); out != "" {
			t.Errorf("%s rendered %q for DialogResume, want empty", name, out)
		}
	}
}

// TestHeaderStructuralEquivalence: the header carries the brand, the cwd and
// (when the approval gate is off) the yolo badge, fits the width it is given,
// and is structurally the same on both surfaces.
func TestHeaderStructuralEquivalence(t *testing.T) {
	for _, autoApprove := range []bool{false, true} {
		v := render.ViewState{Width: 50, Brand: "nib", Cwd: "~/src/project", AutoApprove: autoApprove}
		tokens := []string{"nib", "~/src/project"}
		fps := map[string]fingerprint{}
		for name, p := range presenters() {
			out := p.Header(v)
			assertPreserves(t, name, out, tokens)
			assertFitsWidth(t, name, out, v.Width)
			fps[name] = fingerprintOf(out, tokens)
		}
		assertSameFingerprint(t, fps)
	}
}

// TestReasoningStructuralEquivalence: nothing at all when not loading, and the
// spinner, status verb and trace text in the same order on both surfaces.
func TestReasoningStructuralEquivalence(t *testing.T) {
	const w = 50
	idle := render.ViewState{Loading: false, Spinner: "|", Status: "Working", Reasoning: render.Reasoning{Text: "thinking about it"}}
	for name, p := range presenters() {
		if out := p.Reasoning(idle, w); out != "" {
			t.Errorf("%s rendered %q while not loading, want empty", name, out)
		}
	}

	// collapsedTrace is 8 lines; MaxLines below caps it to the trailing 3
	// (lines 6-8), hiding 5 (lines 1-5). wantHint mirrors exactly what both
	// Reasoning implementations compose from theme.ReasoningMore/Expand, so a
	// hint-composition regression in either surface fails this, not just a
	// generic "some text changed" fingerprint drift.
	collapsedTrace := "line-1\nline-2\nline-3\nline-4\nline-5\nline-6\nline-7\nline-8"
	wantHint := "… 5" + theme.ReasoningMore + theme.ReasoningExpand

	cases := []struct {
		name   string
		state  render.ViewState
		tokens []string
		// absent lists substrings that must NOT survive into the output — the
		// collapsed box's whole point is that the head is dropped, not merely
		// that the tail is present (a bug that showed everything would also
		// pass a tokens-only check).
		absent []string
	}{
		{
			name:   "indicator only",
			state:  render.ViewState{Loading: true, Spinner: "|", Status: "Working"},
			tokens: []string{"|", "Working"},
		},
		{
			name:   "indicator with a reasoning trace",
			state:  render.ViewState{Loading: true, Spinner: "|", Status: "Working", Reasoning: render.Reasoning{Text: "thinking about it"}},
			tokens: []string{"|", "Working", "thinking about it"},
		},
		{
			// Pins CollapsibleBox's tail-anchoring end to end through BOTH
			// presenters, not just inline (which tui/reasoning_test.go already
			// covers via the model). Without this case, full's own collapsing
			// branch was invoked by no test in the suite: TestReasoningStruc-
			// turalEquivalence never set Collapsed/MaxLines, so it only ever
			// exercised the pass-through (uncapped) path, and full.Reasoning
			// could regress independently of inline with nothing to catch it.
			name: "collapsed reasoning trace tails, does not head",
			state: render.ViewState{
				Loading: true, Spinner: "|", Status: "Working",
				Reasoning: render.Reasoning{Text: collapsedTrace, Collapsed: true, MaxLines: 3},
			},
			tokens: []string{"|", "Working", "line-6", "line-7", "line-8", wantHint},
			absent: []string{"line-1", "line-2", "line-3", "line-4", "line-5"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fps := map[string]fingerprint{}
			for name, p := range presenters() {
				out := p.Reasoning(tc.state, w)
				assertPreserves(t, name, out, tc.tokens)
				assertFitsWidth(t, name, out, w)
				for _, tok := range tc.absent {
					if strings.Contains(stripANSI(out), tok) {
						t.Errorf("%s collapsed box leaked hidden content %q: %q", name, tok, out)
					}
				}
				fps[name] = fingerprintOf(out, tc.tokens)
			}
			assertSameFingerprint(t, fps)
		})
	}
}

// footerStates enumerates the footer shapes that actually occur, from the bare
// help line to every row at once. Shared by the structural check and the
// FooterHeight budget check below.
func footerStates() []struct {
	name  string
	state render.ViewState
} {
	help := "tab complete"
	rows := []render.FooterRow{
		{Glyph: "*", Text: "2 jobs running", Kind: render.FooterJobs},
		{Glyph: ">", Text: "1 shell job", Kind: render.FooterShell},
		{Glyph: "~", Text: "loop every 5m", Kind: render.FooterLoops},
		{Glyph: "o", Text: "goal: ship it", Kind: render.FooterGoal},
	}
	return []struct {
		name  string
		state render.ViewState
	}{
		{"help only", render.ViewState{Help: help}},
		{"help and badges", render.ViewState{Help: help, Badges: "12k ctx"}},
		{"new output marker", render.ViewState{Help: help, NewOutput: true}},
		{"error line", render.ViewState{Help: help, Err: "something failed"}},
		{"one job row", render.ViewState{Help: help, Footers: rows[:1]}},
		{"job and loop rows", render.ViewState{Help: help, Footers: []render.FooterRow{rows[0], rows[2]}}},
		{"every row", render.ViewState{Help: help, Badges: "12k ctx", NewOutput: true, Err: "something failed", Footers: rows}},
		{"a row with no glyph", render.ViewState{Help: help, Footers: []render.FooterRow{{Text: "unset kind row"}}}},
	}
}

// TestFooterStructuralEquivalence: the footer's rows are domain data, so both
// surfaces must render the same number of them, in the same order, with the
// same text — however differently they style them.
func TestFooterStructuralEquivalence(t *testing.T) {
	const w = 60
	for _, tc := range footerStates() {
		t.Run(tc.name, func(t *testing.T) {
			var tokens []string
			if tc.state.Help != "" {
				tokens = append(tokens, tc.state.Help)
			}
			if tc.state.Badges != "" {
				tokens = append(tokens, tc.state.Badges)
			}
			if tc.state.Err != "" {
				tokens = append(tokens, tc.state.Err)
			}
			for _, row := range tc.state.Footers {
				tokens = append(tokens, row.Text)
			}
			fps := map[string]fingerprint{}
			for name, p := range presenters() {
				out := p.Footer(tc.state, w)
				assertPreserves(t, name, out, tokens)
				fps[name] = fingerprintOf(out, tokens)
			}
			assertSameFingerprint(t, fps)
		})
	}
}

// TestFooterHeightMatchesFooter is the height-budget guard. The core subtracts
// FooterHeight from the frame to size the viewport, so an answer that disagrees
// with Footer by even one row makes an over-tall frame — which on the alt
// screen scrolls the header off the top every time a job row appears.
func TestFooterHeightMatchesFooter(t *testing.T) {
	for _, w := range []int{20, 60} {
		for _, tc := range footerStates() {
			for name, p := range presenters() {
				got := p.FooterHeight(tc.state, w)
				want := lipgloss.Height(p.Footer(tc.state, w))
				if got != want {
					t.Errorf("%s FooterHeight(%s, w=%d) = %d, but Footer emits %d rows: %q",
						name, tc.name, w, got, want, p.Footer(tc.state, w))
				}
			}
		}
	}
}

// TestFooterHeightGrowsWithRows pins the actual bug this answers: a fixed
// budget of 3 was wrong because the footer is not a fixed height. A session
// with a running sub-agent and a live loop must report more rows than a bare
// help line.
func TestFooterHeightGrowsWithRows(t *testing.T) {
	const w = 60
	bare := render.ViewState{Help: "tab complete"}
	busy := render.ViewState{
		Help:      "tab complete",
		NewOutput: true,
		Err:       "something failed",
		Footers: []render.FooterRow{
			{Glyph: "*", Text: "2 jobs running", Kind: render.FooterJobs},
			{Glyph: "~", Text: "loop every 5m", Kind: render.FooterLoops},
		},
	}
	for name, p := range presenters() {
		lo, hi := p.FooterHeight(bare, w), p.FooterHeight(busy, w)
		if lo != 1 {
			t.Errorf("%s FooterHeight(bare) = %d, want 1", name, lo)
		}
		if hi != 5 {
			t.Errorf("%s FooterHeight(busy) = %d, want 5 (marker + help + error + two rows)", name, hi)
		}
	}
}

// TestContentWidthLeavesRoomForChrome: every presenter must report a content
// width that its own Message output actually respects, and never a width below
// 1 (glamour and Wrap both need a legal budget even on a terminal narrower
// than the chrome).
func TestContentWidthLeavesRoomForChrome(t *testing.T) {
	roles := []render.Role{render.RoleUser, render.RoleAssistant, render.RoleAgent, render.RoleTool, render.RoleError, render.RoleNone}
	for name, p := range presenters() {
		for _, role := range roles {
			for _, w := range []int{1, 3, 40, 200} {
				got := p.ContentWidth(role, w)
				if got < 1 {
					t.Errorf("%s ContentWidth(%q, %d) = %d, want at least 1", name, role, w, got)
				}
				if got > w {
					t.Errorf("%s ContentWidth(%q, %d) = %d, wider than the frame", name, role, w, got)
				}
			}
		}
	}
}

// TestContentWidthMatchesRenderedPrefix ties ContentWidth to what Message
// actually lays out: content rendered at the reported width must fit the frame
// once the presenter has added its own chrome. This is the property the model
// relies on when it pre-renders markdown (I2) instead of reconstructing one
// surface's prefix for both.
func TestContentWidthMatchesRenderedPrefix(t *testing.T) {
	const w = 48
	for name, p := range presenters() {
		for _, role := range []render.Role{render.RoleUser, render.RoleAssistant, render.RoleAgent, render.RoleError} {
			cw := p.ContentWidth(role, w)
			content := strings.Repeat("x", cw)
			out := p.Message(render.Message{Role: role, Content: content, Label: "label"}, render.RoleNone, w)
			assertFitsWidth(t, name+"/"+string(role), out, w)
		}
	}
}

// TestFrameContainsEveryPiece is Task 10a's composition-point guard: every
// block a frame is made of — the header's brand/cwd, the body (the rendered
// transcript viewport, standing in here for whatever Messages/Reasoning/
// Dialogs produced it), the composer, and the footer's help/badges/error/job
// rows — must survive into Frame's output on both surfaces. This is what
// closes the review finding that no Presenter method received a height and
// no method could see body/composer/footer/chrome all at once.
//
// It does NOT assert an (w, h) clamp: in this task neither presenter clamps
// to h — both still stack, exactly as they did before Frame existed, so a
// body taller than h passes through unclamped on both surfaces. That is
// deliberate (see full.Frame's doc comment) and asserting a clamp here would
// pin behaviour Task 11 is the one that adds.
func TestFrameContainsEveryPiece(t *testing.T) {
	const w, h = 60, 40
	v := render.ViewState{
		Width: w, Height: h,
		Brand: "nib", Cwd: "~/src/project",
		Help: "tab complete", Badges: "12k ctx", Err: "something failed",
		Footers: []render.FooterRow{{Glyph: "*", Text: "1 job running", Kind: render.FooterJobs}},
	}
	body := "BODY-MARKER-ONE\nBODY-MARKER-TWO"
	composer := "COMPOSER-MARKER"

	for name, p := range presenters() {
		t.Run(name, func(t *testing.T) {
			header := p.Header(v)
			footer := p.Footer(v, w)
			out := p.Frame(v, header, body, composer, footer, w, h)

			tokens := []string{
				"nib", "~/src/project", // header
				"BODY-MARKER-ONE", "BODY-MARKER-TWO", // body
				"COMPOSER-MARKER",                                              // composer
				"tab complete", "12k ctx", "something failed", "1 job running", // footer
			}
			assertPreserves(t, name, out, tokens)

			if !strings.Contains(out, composer) {
				t.Errorf("%s Frame dropped the composer entirely: %q", name, out)
			}
		})
	}
}

// TestFrameStructuralEquivalence: both surfaces must agree on how the four
// pieces relate to each other structurally (how many blocks, in what order),
// even though Task 10a deliberately keeps both stacking rather than framing —
// the point of this task is that the restructuring is behaviour-preserving on
// both surfaces, provably so via the same fingerprint comparison the rest of
// this suite uses.
func TestFrameStructuralEquivalence(t *testing.T) {
	const w, h = 60, 40
	v := render.ViewState{Width: w, Height: h, Brand: "nib", Cwd: "~/src/project", Help: "tab complete"}
	body := "BODY-MARKER"
	composer := "COMPOSER-MARKER"
	tokens := []string{"nib", "~/src/project", "BODY-MARKER", "COMPOSER-MARKER", "tab complete"}

	fps := map[string]fingerprint{}
	for name, p := range presenters() {
		header := p.Header(v)
		footer := p.Footer(v, w)
		out := p.Frame(v, header, body, composer, footer, w, h)
		fps[name] = fingerprintOf(out, tokens)
	}
	assertSameFingerprint(t, fps)
}

// TestAllPresentersMarkDialogSelection: the selected option must be visually
// distinguishable in every surface, however each one chooses to mark it.
// Moved here from Phase 2 Task 7 by controller ruling: it asserts behaviour
// that only exists once Phase 3 Task 11 (the ask_user dialog) lands.
func TestAllPresentersMarkDialogSelection(t *testing.T) {
	d := render.Dialog{
		Kind:  render.DialogAsk,
		Title: "pick one",
		// Options is []render.DialogOption{Text, Emphasis} — the explicit shape
		// adopted in Phase 2 Task 6's fix round, replacing positional styling.
		Options: []render.DialogOption{
			{Text: "alpha"}, {Text: "beta"}, {Text: "gamma"},
		},
		Selected: 1,
	}
	for name, p := range presenters() {
		t.Run(name, func(t *testing.T) {
			out := p.Dialog(d, 80)
			lines := strings.Split(out, "\n")
			var betaLine, alphaLine string
			for _, l := range lines {
				if strings.Contains(l, "beta") {
					betaLine = l
				}
				if strings.Contains(l, "alpha") {
					alphaLine = l
				}
			}
			if betaLine == "" || alphaLine == "" {
				t.Fatalf("%s did not render all options: %q", name, out)
			}
			if betaLine == strings.Replace(alphaLine, "alpha", "beta", 1) {
				t.Errorf("%s renders the selected option identically to an unselected one: %q", name, betaLine)
			}
		})
	}
}
