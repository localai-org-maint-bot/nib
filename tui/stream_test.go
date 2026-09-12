package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// TestReasoningDeltaAccumulatesInOrder feeds a sequence of streaming reasoning
// deltas through Update and asserts they land in m.reasoning concatenated in
// the order they arrived — a delta pipeline that reorders or drops chunks
// would produce a trace nobody could trust.
func TestReasoningDeltaAccumulatesInOrder(t *testing.T) {
	m := Model{
		viewport:  viewport.New(80, 20),
		width:     80,
		loading:   true,
		presenter: testPresenter(),
	}

	deltas := []string{"The", " quick", " brown", " fox", " jumps"}
	var next tea.Model = m
	for _, d := range deltas {
		next, _ = next.(Model).Update(reasoningDeltaMsg(d))
	}

	got := next.(Model).reasoning
	want := strings.Join(deltas, "")
	if got != want {
		t.Fatalf("reasoning = %q, want %q (deltas out of order or dropped)", got, want)
	}
}

// TestReasoningDeltaTailsWhenCollapsed streams enough deltas (one per line) to
// exceed theme.ReasoningMaxLines and asserts the collapsed box still shows the
// newest lines, mirroring TestReasoningCollapsedByDefaultTailsTheTrace but
// arriving via the streaming path instead of a single OnReasoning write.
func TestReasoningDeltaTailsWhenCollapsed(t *testing.T) {
	m := Model{
		viewport:           viewport.New(80, 20),
		width:              80,
		loading:            true,
		presenter:          testPresenter(),
		reasoningCollapsed: true,
	}

	var next tea.Model = m
	for i := 0; i < 20; i++ {
		line := "trace line " + string(rune('a'+i))
		next, _ = next.(Model).Update(reasoningDeltaMsg(line + "\n"))
	}

	out := next.(Model).viewport.View()
	if !strings.Contains(out, "trace line t") {
		t.Error("collapsed box does not show the newest streamed line")
	}
	if strings.Contains(out, "trace line a") {
		t.Error("collapsed box is showing the oldest streamed line; it should tail, not head")
	}
}

// TestReasoningBoundaryResetsStreamAccumulation is the precedence test: a
// step-boundary OnReasoning write (reasoningMsg) is authoritative for the step
// that just ended, but it must NOT become a prefix that the next step's
// streamed deltas get appended onto — otherwise every step after the first
// would duplicate the previous step's complete text.
func TestReasoningBoundaryResetsStreamAccumulation(t *testing.T) {
	m := Model{
		viewport:  viewport.New(80, 20),
		width:     80,
		loading:   true,
		presenter: testPresenter(),
	}

	var next tea.Model = m
	// Step 1 streams in two chunks.
	next, _ = next.(Model).Update(reasoningDeltaMsg("ab"))
	next, _ = next.(Model).Update(reasoningDeltaMsg("cd"))
	if got := next.(Model).reasoning; got != "abcd" {
		t.Fatalf("mid-step reasoning = %q, want %q", got, "abcd")
	}

	// Step 1 ends: OnReasoning fires with the complete, authoritative block.
	next, _ = next.(Model).Update(reasoningMsg("STEP1-COMPLETE"))
	if got := next.(Model).reasoning; got != "STEP1-COMPLETE" {
		t.Fatalf("post-boundary reasoning = %q, want %q", got, "STEP1-COMPLETE")
	}

	// Step 2 starts streaming. Its first delta must start a fresh trace, not
	// append onto step 1's authoritative text.
	next, _ = next.(Model).Update(reasoningDeltaMsg("xy"))
	if got := next.(Model).reasoning; got != "xy" {
		t.Fatalf("first delta after boundary = %q, want %q (must not carry over the prior step's text)", got, "xy")
	}

	// Further step-2 deltas keep accumulating normally.
	next, _ = next.(Model).Update(reasoningDeltaMsg("z"))
	if got := next.(Model).reasoning; got != "xyz" {
		t.Fatalf("second delta after boundary = %q, want %q", got, "xyz")
	}
}

// TestListenReasoningDeltaCoalescesABurst confirms the listener drains
// whatever is already queued on the channel into a single message rather than
// returning one message per delta — the mechanism that keeps a fast token
// stream from forcing an updateViewport per token.
func TestListenReasoningDeltaCoalescesABurst(t *testing.T) {
	m := Model{
		ctx:                context.Background(),
		reasoningDeltaChan: make(chan string, 8),
	}
	m.reasoningDeltaChan <- "a"
	m.reasoningDeltaChan <- "b"
	m.reasoningDeltaChan <- "c"

	msg := m.listenReasoningDelta()()
	got, ok := msg.(reasoningDeltaMsg)
	if !ok {
		t.Fatalf("listenReasoningDelta() returned %T, want reasoningDeltaMsg", msg)
	}
	if string(got) != "abc" {
		t.Fatalf("coalesced delta = %q, want %q (a burst must collapse into one message, in order)", string(got), "abc")
	}

	// The channel is drained: a second call blocks until fed again, i.e. it
	// does not re-deliver anything left over from the burst.
	select {
	case m.reasoningDeltaChan <- "d":
	default:
		t.Fatal("channel unexpectedly full after drain")
	}
	msg2 := m.listenReasoningDelta()()
	got2 := msg2.(reasoningDeltaMsg)
	if string(got2) != "d" {
		t.Fatalf("post-drain delta = %q, want %q", string(got2), "d")
	}
}
