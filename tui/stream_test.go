package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// delta wraps one streamed reasoning chunk as the batch-of-one Update expects
// from listenReasoningEvents in the common case.
func delta(text string) reasoningEventsMsg {
	return reasoningEventsMsg{{kind: reasoningEventDelta, text: text}}
}

// boundary wraps one step-boundary (complete) reasoning block the same way.
func boundary(text string) reasoningEventsMsg {
	return reasoningEventsMsg{{kind: reasoningEventBoundary, text: text}}
}

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
		next, _ = next.(Model).Update(delta(d))
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
		next, _ = next.(Model).Update(delta(line + "\n"))
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
// step-boundary OnReasoning write is authoritative for the step that just
// ended, but it must NOT become a prefix that the next step's streamed
// deltas get appended onto — otherwise every step after the first would
// duplicate the previous step's complete text.
func TestReasoningBoundaryResetsStreamAccumulation(t *testing.T) {
	m := Model{
		viewport:  viewport.New(80, 20),
		width:     80,
		loading:   true,
		presenter: testPresenter(),
	}

	var next tea.Model = m
	// Step 1 streams in two chunks.
	next, _ = next.(Model).Update(delta("ab"))
	next, _ = next.(Model).Update(delta("cd"))
	if got := next.(Model).reasoning; got != "abcd" {
		t.Fatalf("mid-step reasoning = %q, want %q", got, "abcd")
	}

	// Step 1 ends: OnReasoning fires with the complete, authoritative block.
	next, _ = next.(Model).Update(boundary("STEP1-COMPLETE"))
	if got := next.(Model).reasoning; got != "STEP1-COMPLETE" {
		t.Fatalf("post-boundary reasoning = %q, want %q", got, "STEP1-COMPLETE")
	}
	if !next.(Model).reasoningResetPending {
		t.Fatal("boundary must arm reasoningResetPending for the next step's first delta")
	}

	// Step 2 starts streaming. Its first delta must start a fresh trace, not
	// append onto step 1's authoritative text.
	next, _ = next.(Model).Update(delta("xy"))
	if got := next.(Model).reasoning; got != "xy" {
		t.Fatalf("first delta after boundary = %q, want %q (must not carry over the prior step's text)", got, "xy")
	}

	// Further step-2 deltas keep accumulating normally.
	next, _ = next.(Model).Update(delta("z"))
	if got := next.(Model).reasoning; got != "xyz" {
		t.Fatalf("second delta after boundary = %q, want %q", got, "xyz")
	}
}

// TestConsecutiveBoundariesWithNoDeltaBetween: two step-boundary events with
// no delta in between (e.g. two tool-selection steps in a row that streamed
// no reasoning) must leave the SECOND boundary's text showing — not a mix of
// both, and not stuck on the first. Delivered as one batch, exactly as
// listenReasoningEvents would hand them to Update after draining two queued
// boundary events.
func TestConsecutiveBoundariesWithNoDeltaBetween(t *testing.T) {
	m := Model{
		viewport:  viewport.New(80, 20),
		width:     80,
		loading:   true,
		presenter: testPresenter(),
	}

	batch := reasoningEventsMsg{
		{kind: reasoningEventBoundary, text: "STEP1-COMPLETE"},
		{kind: reasoningEventBoundary, text: "STEP2-COMPLETE"},
	}
	next, _ := m.Update(batch)

	got := next.(Model).reasoning
	if got != "STEP2-COMPLETE" {
		t.Fatalf("reasoning = %q, want %q (the later boundary must win, not a mix of both)", got, "STEP2-COMPLETE")
	}
	// A delta right after must start fresh from STEP2's text, not append.
	next2, _ := next.(Model).Update(delta("more"))
	if got := next2.(Model).reasoning; got != "more" {
		t.Fatalf("delta after two boundaries = %q, want %q", got, "more")
	}
}

// TestReasoningResetPendingClearsAtTurnEnd is the flag-leakage regression: a
// boundary event with no following delta (the turn ends right there) must
// leave neither stale text nor an armed reset flag for the NEXT turn to
// inherit. responseMsg is the normal (non-interrupted) turn-end path.
func TestReasoningResetPendingClearsAtTurnEnd(t *testing.T) {
	m := Model{
		viewport:  viewport.New(80, 20),
		width:     80,
		loading:   true,
		presenter: testPresenter(),
	}

	next, _ := m.Update(boundary("FINAL-STEP"))
	if !next.(Model).reasoningResetPending {
		t.Fatal("setup: boundary should have armed reasoningResetPending")
	}

	next, _ = next.(Model).Update(responseMsg{content: "the answer"})
	got := next.(Model)
	if got.reasoning != "" {
		t.Fatalf("reasoning after turn end = %q, want empty (no stale boundary text)", got.reasoning)
	}
	if got.reasoningResetPending {
		t.Fatal("reasoningResetPending leaked past turn end into the next turn")
	}
}

// TestReasoningResetPendingClearsOnInterrupt: Ctrl+C mid-stream ends the turn
// via the same responseMsg path but with a context.Canceled error. The flag
// must not survive that either, or the NEXT turn's first delta would
// silently inherit reset-then-append state from the interrupted one.
func TestReasoningResetPendingClearsOnInterrupt(t *testing.T) {
	m := Model{
		viewport:  viewport.New(80, 20),
		width:     80,
		loading:   true,
		presenter: testPresenter(),
	}

	// Mid-stream: a delta, then a boundary (arms the flag), simulating an
	// interrupt landing right after a step completed.
	next, _ := m.Update(delta("partial thought"))
	next, _ = next.(Model).Update(boundary("STEP-COMPLETE"))
	if !next.(Model).reasoningResetPending {
		t.Fatal("setup: boundary should have armed reasoningResetPending")
	}

	next, _ = next.(Model).Update(responseMsg{err: context.Canceled})
	got := next.(Model)
	if got.reasoning != "" {
		t.Fatalf("reasoning after interrupt = %q, want empty", got.reasoning)
	}
	if got.reasoningResetPending {
		t.Fatal("reasoningResetPending leaked past an interrupted turn")
	}
}

// TestReasoningResetPendingClearsOnPark exercises the OTHER turn-boundary
// reset site (parkMsg, not responseMsg): a boundary with no following delta,
// where the run parks instead of fully ending. Same requirement — no stale
// text, no leaked flag — through a different code path.
func TestReasoningResetPendingClearsOnPark(t *testing.T) {
	m := Model{
		viewport:  viewport.New(80, 20),
		textarea:  textarea.New(),
		width:     80,
		loading:   true,
		presenter: testPresenter(),
	}

	next, _ := m.Update(boundary("FINAL-STEP"))
	if !next.(Model).reasoningResetPending {
		t.Fatal("setup: boundary should have armed reasoningResetPending")
	}

	next, _ = next.(Model).Update(parkMsg{parked: true, reply: "parked reply"})
	got := next.(Model)
	if got.reasoning != "" {
		t.Fatalf("reasoning after park = %q, want empty (no stale boundary text)", got.reasoning)
	}
	if got.reasoningResetPending {
		t.Fatal("reasoningResetPending leaked past a park into whatever runs next")
	}
}

// TestListenReasoningEventsCoalescesABurst confirms the listener drains
// whatever is already queued on the channel into a single message rather than
// returning one message per event — the mechanism that keeps a fast token
// stream from forcing an updateViewport per token — and that it preserves
// arrival order across a mix of delta and boundary kinds.
func TestListenReasoningEventsCoalescesABurst(t *testing.T) {
	m := Model{
		ctx:           context.Background(),
		reasoningChan: make(chan reasoningEvent, 8),
	}
	m.reasoningChan <- reasoningEvent{kind: reasoningEventDelta, text: "a"}
	m.reasoningChan <- reasoningEvent{kind: reasoningEventDelta, text: "b"}
	m.reasoningChan <- reasoningEvent{kind: reasoningEventBoundary, text: "ab-complete"}

	msg := m.listenReasoningEvents()()
	got, ok := msg.(reasoningEventsMsg)
	if !ok {
		t.Fatalf("listenReasoningEvents() returned %T, want reasoningEventsMsg", msg)
	}
	want := reasoningEventsMsg{
		{kind: reasoningEventDelta, text: "a"},
		{kind: reasoningEventDelta, text: "b"},
		{kind: reasoningEventBoundary, text: "ab-complete"},
	}
	if len(got) != len(want) {
		t.Fatalf("coalesced batch has %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d = %+v, want %+v (order not preserved)", i, got[i], want[i])
		}
	}

	// The channel is drained: a second call blocks until fed again, i.e. it
	// does not re-deliver anything left over from the burst.
	select {
	case m.reasoningChan <- reasoningEvent{kind: reasoningEventDelta, text: "d"}:
	default:
		t.Fatal("channel unexpectedly full after drain")
	}
	msg2 := m.listenReasoningEvents()()
	got2 := msg2.(reasoningEventsMsg)
	if len(got2) != 1 || got2[0].text != "d" {
		t.Fatalf("post-drain batch = %+v, want a single %q delta", got2, "d")
	}
}

// TestListenReasoningEventsPreservesOrderUnderRealConcurrency exercises the
// REAL reasoningChan and the REAL listenReasoningEvents goroutine path (not
// Update called synchronously in-process): a producer goroutine sends a mix
// of delta and boundary events with real scheduling in between sends, while
// this test repeatedly invokes the listener's returned tea.Cmd exactly the
// way bubbletea would after each re-arm, and flattens the batches it gets
// back. This is the scenario the single-channel design exists for: it is the
// two-channel version of this same test that could have shown a boundary
// overtaking a still-buffered final delta.
//
// It does not stand up a full tea.Program (bubbletea's own per-Cmd goroutine
// dispatch is not exercised here) — that infrastructure doesn't exist in this
// package's test harness and adding it is out of scope for this fix. What it
// does prove: with everything funneled through one channel, concurrent
// producer/consumer scheduling cannot reorder what Update eventually sees,
// because there is only one FIFO queue in play.
func TestListenReasoningEventsPreservesOrderUnderRealConcurrency(t *testing.T) {
	m := Model{
		ctx:           context.Background(),
		reasoningChan: make(chan reasoningEvent, 4), // small: forces real blocking/handoff
	}

	want := reasoningEventsMsg{
		{kind: reasoningEventDelta, text: "Let"},
		{kind: reasoningEventDelta, text: " me"},
		{kind: reasoningEventDelta, text: " think"},
		{kind: reasoningEventBoundary, text: "Let me think"},
		{kind: reasoningEventDelta, text: "Next"},
		{kind: reasoningEventDelta, text: " step"},
		{kind: reasoningEventBoundary, text: "Next step"},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, ev := range want {
			m.reasoningChan <- ev
		}
	}()

	var got reasoningEventsMsg
	for len(got) < len(want) {
		msg := m.listenReasoningEvents()()
		batch, ok := msg.(reasoningEventsMsg)
		if !ok {
			t.Fatalf("listenReasoningEvents() returned %T, want reasoningEventsMsg", msg)
		}
		got = append(got, batch...)
	}
	<-done

	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d = %+v, want %+v — producer order not preserved under real concurrency", i, got[i], want[i])
		}
	}
}
