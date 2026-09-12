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
	// HugNext is meaningful for RoleAgent only: true when the next raw message
	// continues this same agent's thread (a run of agent_tool/agent_result
	// lines rendered separately by the model, never through Message). A
	// Presenter must omit its own trailing separator in that case — the
	// thread run that follows hugs it instead. The model computes this by
	// looking ahead at the next raw message, which a Presenter never sees, so
	// it cannot be derived from prev/Role alone.
	HugNext bool
}

// Reasoning is the model's in-progress reasoning trace: the text beneath the
// working indicator (spinner + status), which lives on ViewState.Spinner and
// ViewState.Status — kept there only, so there is exactly one source of
// truth for them.
type Reasoning struct {
	Text      string
	Collapsed bool
	MaxLines  int
}

// DialogKind identifies which modal dialog is being shown.
type DialogKind int

const (
	DialogApproval DialogKind = iota
	DialogAsk
	DialogResume
)

// DialogOption is one line of a Dialog's choice menu. Emphasis marks it as an
// actionable key (styled as such); a false Emphasis is a dimmer, non-key hint
// line (e.g. the approval block's "[n] no · [e] edit" line, which sits among
// three actionable choices but isn't one itself).
type DialogOption struct {
	Text     string
	Emphasis bool
}

// Dialog is a modal prompt (tool approval, ask, resume) awaiting user input.
//
// DialogAsk carries its entire pre-rendered block in Title (the ask/multi-
// select question-and-options block is domain logic — parsing chat.AskRequest
// and formatting numbered/checkbox options — that stays in tui, same
// precedent as markdown content for Message); a Presenter for that kind just
// places Title verbatim.
//
// DialogApproval uses the rest of the fields:
//   - Rows holds the argument card: one [key, value] pair per structured
//     argument (chat.ToolArgRows). RowsUnstructured, when true, means Rows
//     instead holds exactly one entry whose second element is a raw prose
//     block (the tool call formatted as text) for a tool chat.ToolArgRows
//     doesn't recognize — a Presenter wraps and dims it instead of laying out
//     a key/value table. This flag exists so that distinction is explicit in
//     the type, rather than inferred from an empty Rows[0][0] key (which is
//     not actually guaranteed unique: a tool whose JSON arguments contain a
//     literal "" key would collide with that convention).
//   - Hint is the tool call's captured reasoning (if any), rendered wrapped
//     beneath the rows.
//   - Options is the choice menu: 4 entries (once / always / this-turn /
//     deny-edit, matching the on-screen approval prompt) in the normal case,
//     or a single entry (the edit-mode hint) while editing. A Presenter
//     should not assume only those two counts occur — render whatever list
//     it's given — but may special-case exactly 4 to reproduce the classic
//     approval layout (a blank gutter line before the menu).
type Dialog struct {
	Kind             DialogKind
	Title            string
	Options          []DialogOption
	Selected         int
	Checked          []bool      // multi-select state; nil for single-select
	Rows             [][2]string // key/value detail rows (approval argument cards)
	RowsUnstructured bool        // true: Rows is a single prose row, not a key/value table
	Hint             string
}

// FooterRow is one line of the footer's job-status area (active sub-agent
// jobs, shell jobs, cron loops, the active goal). It carries data, not
// pixels: Glyph is the marker rune (e.g. theme.Loop), Text is the already-
// composed but UNSTYLED line. A Presenter decides the styling; the tui-side
// callers that build these (tui/agents.go, tui/shelljobs.go, tui/loops.go,
// tui/goal.go) must not depend on any Presenter or style types, only produce
// plain data, since a Presenter must never import their argument types
// (agentJob, *loop.Registry, wizmcp.ShellJobInfo).
type FooterRow struct {
	Glyph string
	Text  string
}

// ViewState is the read-only projection of Model state a Presenter renders
// from. It carries no behaviour — presenters read it and produce strings.
//
// The model builds one ViewState per frame (see tui/model.go's viewState
// method) with every field populated — Messages/Reasoning/Dialog included,
// even though the inline Presenter's Header/Footer never read them — so nothing
// here is silently nil for a Presenter that composes a whole alt-screen frame
// from one ViewState rather than being driven block-by-block.
//
// Fields from NewOutput onward were added by Task 6, which is the first to
// actually build a ViewState (Task 5 declared the struct with no call sites).
// The original Task 5 contract had no way to reach the model's viewport or
// its job-registry state from a Presenter, both of which the pre-existing
// hand-rolled Footer rendering needs:
//
//   - NewOutput: the presenter cannot see m.viewport, so the model resolves
//     "is there unread content below the fold" (showingViewport &&
//     !m.viewport.AtBottom()) itself and passes the answer through.
//   - Err: the plain (unstyled) text of the model's last error, or "" for
//     none. The presenter applies the error glyph and style.
//   - Footers: plain {Glyph, Text} data for the active-jobs/shell-jobs/loops/
//     goal footer rows (see FooterRow) — the presenter styles and joins
//     whichever are present, in order.
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

	NewOutput bool
	Err       string
	Footers   []FooterRow
}
