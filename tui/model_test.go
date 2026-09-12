package tui

import "github.com/mudler/nib/tui/render/inline"

// newTestModel builds a Model for tests that need more than the zero value a
// bare Model{} literal gives, while still guaranteeing a non-nil presenter —
// the same guarantee every production construction path (NewModel, always
// called with a Presenter from app.go) carries. Model.presenter used to have
// a nil fallback in renderer() that quietly rendered the inline surface for
// any Model built without one; that fallback is gone, so a test that reaches
// View() or updateViewport() must build its Model through here instead of a
// bare Model{} literal.
//
// fields is applied on top of a Model whose only preset value is presenter;
// set fields.presenter explicitly to exercise a different Presenter.
func newTestModel(fields Model) Model {
	if fields.presenter == nil {
		fields.presenter = inline.New()
	}
	// Give the projection cache the same starting revision NewModel does, so
	// tests exercise the cached path rather than the uncached nil-cache
	// fallback — a zero-valued cache is itself the bug messageProjCache's
	// rev: -1 exists to prevent.
	if fields.msgViewCache == nil {
		fields.msgViewCache = &messageProjCache{rev: -1}
	}
	return fields
}

// withMessages appends transcript entries through appendMessage — the single
// choke point that bumps msgRev and so keeps msgViewCache's invalidation key
// correct — and returns the Model, so a test can seed a transcript in one
// expression. Tests used to assign m.messages directly, which is exactly the
// invariant appendMessage's doc comment forbids: without the revision bump a
// non-empty transcript could project as nil. Routing setup through here makes
// that invariant enforceable rather than merely asserted in a comment.
func withMessages(m Model, msgs ...ChatMessage) Model {
	m.appendMessage(msgs...)
	return m
}
