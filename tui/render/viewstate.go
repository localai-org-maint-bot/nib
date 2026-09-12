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

// Reasoning is the model's in-progress reasoning trace.
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
}
