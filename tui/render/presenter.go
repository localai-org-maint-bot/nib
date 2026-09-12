package render

// Caps declares what a Presenter's surface supports, so the shared core can
// adapt behaviour (e.g. mouse-driven scrolling) without a presenter needing
// to know about the other surface.
type Caps struct {
	AltScreen bool
	Mouse     bool
}

// Presenter renders a ViewState (and its parts) into strings for one surface
// — the inline fzf-style widget or the full-screen alt-screen mode. The
// shared core drives a Presenter; a Presenter holds no state of its own.
//
// Reasoning takes the full ViewState (not just its Reasoning field) because
// the working indicator it renders — the spinner frame and status verb —
// lives on ViewState.Spinner/Status, the single source of truth for both
// (rather than duplicating them onto the narrower Reasoning type). A
// Presenter should render nothing when !v.Loading.
type Presenter interface {
	Caps() Caps
	Header(v ViewState) string
	Message(m Message, prev Role, w int) string
	Reasoning(v ViewState, w int) string
	Dialog(d Dialog, w int) string
	Footer(v ViewState, w int) string
}
