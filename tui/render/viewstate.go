package render

// Role identifies the speaker or origin of a Message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleAgent     Role = "agent"
	RoleTool      Role = "tool"
	RoleError     Role = "error"
	RoleNone      Role = ""
)

// Message is a single rendered chat entry.
type Message struct {
	Role      Role
	Content   string
	Name      string // tool name, for RoleTool
	Arguments string // marshaled call args, for RoleTool
	AgentID   string
}

// Reasoning is the model's in-progress reasoning trace, plus the working
// indicator shown above it.
//
// Status and Spinner carry the same information as ViewState's own Status and
// Spinner fields. They are duplicated here — rather than having Presenter's
// Reasoning method take a ViewState — because the working line (spinner frame
// + status verb) and the reasoning trace beneath it render as one block, and
// Message/Reasoning/Dialog intentionally take their own narrow types instead
// of the whole ViewState (only Header/Footer do). Added by Task 6; not in the
// original Task 5 contract.
type Reasoning struct {
	Text      string
	Collapsed bool
	MaxLines  int
	Status    string // status verb shown beside the spinner (already resolved, e.g. "thinking")
	Spinner   string // the model's rendered spinner frame (bubbles/spinner.View())
}

// DialogKind identifies which modal dialog is being shown.
type DialogKind int

const (
	DialogApproval DialogKind = iota
	DialogAsk
	DialogResume
)

// Dialog is a modal prompt (tool approval, ask, resume) awaiting user input.
type Dialog struct {
	Kind     DialogKind
	Title    string
	Options  []string
	Selected int
	Checked  []bool      // multi-select state; nil for single-select
	Rows     [][2]string // key/value detail rows (approval argument cards)
	Hint     string
}

// ViewState is the read-only projection of Model state a Presenter renders
// from. It carries no behaviour — presenters read it and produce strings.
//
// Fields below Badges were added by Task 6, which is the first to actually
// build a ViewState (Task 5 declared the struct with no call sites). The
// original Task 5 contract had no way to reach the model's viewport or its
// job-registry state from a Presenter, both of which the pre-existing
// hand-rolled Footer rendering needs:
//
//   - NewOutput: the presenter cannot see m.viewport, so the model resolves
//     "is there unread content below the fold" (showingViewport &&
//     !m.viewport.AtBottom()) itself and passes the answer through.
//   - Err: the plain (unstyled) text of the model's last error, or "" for
//     none. The presenter applies the error glyph and style.
//   - JobsFooter, ShellJobsFooter, LoopsFooter, GoalFooter: these come from
//     tui-local renderJobsFooter/renderShellJobsFooter/renderLoopsFooter/
//     renderGoalFooter, which take model/domain types (agentJob, ShellJobInfo,
//     *loop.Registry) a Presenter must never depend on. The model calls them
//     and passes the already-rendered, already-styled strings (or "" to omit
//     a row); the presenter only decides spacing between non-empty ones.
type ViewState struct {
	Width       int
	Height      int
	Cwd         string
	Brand       string
	AutoApprove bool
	Loading     bool
	Status      string
	Spinner     string
	Messages    []Message
	Reasoning   Reasoning
	Dialog      *Dialog
	Help        string
	Badges      string

	NewOutput       bool
	Err             string
	JobsFooter      string
	ShellJobsFooter string
	LoopsFooter     string
	GoalFooter      string
}
