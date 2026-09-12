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
// ContentWidth exists because some content is rendered before it reaches a
// Presenter: markdown is glamour output, and glamour is width-cached state the
// model owns, so the model must know at what width to render. That width is
// whatever this surface's chrome leaves, which only the Presenter knows. The
// model asks for the number rather than for the prefix string: a surface whose
// chrome is not a literal per-line prefix (a frame, a hanging gutter) can still
// answer a width, and the model never has to measure chrome it did not compose.
//
// FooterHeight exists for the same reason in the other direction: the core
// budgets the viewport's height by subtracting the footer, and Footer emits
// anywhere from one row (the help line alone) to seven (new-output marker,
// error line, four job-status rows). A fixed guess makes an over-tall frame,
// which on the alt screen scrolls the header off the top. It must agree with
// Footer exactly, for the same ViewState and width.
type Presenter interface {
	Caps() Caps
	Header(v ViewState) string
	Message(m Message, prev Role, w int) string
	ContentWidth(role Role, w int) int
	Reasoning(v ViewState, w int) string
	Dialog(d Dialog, w int) string
	Footer(v ViewState, w int) string
	FooterHeight(v ViewState, w int) int
}
