package theme

// Microcopy — calm, lowercase, no wizard metaphor, no emoji.
const (
	BrandName = "nib"

	// LabelYouText labels the user's own chat messages (paired with LabelYou,
	// the style, in theme.go).
	LabelYouText = "you"

	HelpDefault      = "enter send · ctrl+y use command · G/end newest · esc exit"
	HelpApproval     = "1 once · 2 always · 3 this turn · n no · e edit · esc deny"
	HelpApprovalEdit = "enter submit · esc cancel"
	ApproveEditHint  = "describe the change · enter submit · esc cancel"

	// NewOutputText is the footer marker shown when the viewport is scrolled up
	// and content has arrived below the fold. Composed with NewOutputGlyph by
	// NewOutputMarker() in theme.go — the glyph is swappable, this text isn't.
	NewOutputText = "new output"

	// The numbered approval menu. Line 2 is dynamic — the TUI composes
	// ApproveAlwaysPrefix + chat.GrantScope(...) + ApproveAlwaysSuffix.
	ApproveOnce         = "[1] run it once"
	ApproveAlwaysPrefix = "[2] always allow "
	ApproveAlwaysSuffix = "  (this session)"
	ApproveTurn         = "[3] yes to everything this turn"
	ApproveDenyEdit     = "[n] no · [e] edit"

	EmptyTagline = "a calm assistant for your terminal."
	EmptyTryLead = "try:"
	EmptySlash   = "type /  for skills, agents & commands"
	SlashHint    = "/ for skills"
	Starting     = "starting…"

	CLIWelcome = "a calm assistant for your terminal."
	CLIExit    = "ctrl+c or 'exit' to leave · 'help' for commands"

	// Shown when a CLI approval prompt gets no answer at all. A closed stdin
	// (the piped one-shot idiom) and a cancelled run are both "nobody
	// decided", which is not a yes, so the call is denied.
	CLIDeniedNoInput  = "denied: stdin closed, nobody left to approve this"
	CLIDeniedNoAnswer = "denied: no answer (the run was cancelled)"

	// Shown when --yolo / NIB_YOLO auto-approves every tool call. The header
	// carries the compact badge; the CLI prints the fuller notice at startup.
	YoloBadge  = "yolo"
	YoloNotice = "yolo — auto-approving every tool call (no prompts)"

	// StatusRunning is shown between an approved tool call and its result.
	StatusRunning = "running…"

	// Reasoning box copy. A collapsed box shows the trailing
	// ReasoningMaxLines lines of the live trace; the TUI composes the hint
	// line as "… " + n + ReasoningMore + ReasoningExpand.
	ReasoningMore     = " more · "
	ReasoningExpand   = "ctrl+r expand"
	ReasoningCollapse = "ctrl+r collapse"

	// ask_user dialog copy (Phase 3 Task 11). HelpAsk is the footer help line
	// while a question is pending; the AskHint* lines sit beneath the option
	// list itself and, unlike HelpAsk, always mention the free-text escape
	// hatch (typing instead of picking), since that's the one thing every ask
	// dialog offers regardless of how it's answered.
	HelpAsk             = "up/down move · enter pick · esc cancel"
	AskHintSingleSelect = "up/down move · enter pick · or type your own answer"
	AskHintMultiSelect  = "up/down move · space toggle · enter confirm · or type your own answer"
	AskHintFreeText     = "type your answer"
)

// ReasoningMaxLines is how many trailing lines a collapsed reasoning box
// shows.
const ReasoningMaxLines = 5

// CLIApprovePrompt builds the line-based CLI approval prompt (the TUI uses
// the numbered single-key menu instead). alwaysScope describes what `a`
// grants for this call — e.g. "`git …`", "any bash command", or a tool name.
func CLIApprovePrompt(alwaysScope string) string {
	return "y yes · a always (" + alwaysScope + ") · all this turn · n no · or type a change"
}

// Status verbs shown while the agent works.
const (
	VerbThinking = "thinking"
	VerbWorking  = "working"
	VerbReading  = "reading"
)

// EmptyExamples are the sample prompts shown on the first-run empty state.
var EmptyExamples = []string{
	"what changed in the last commit?",
	"undo my last git commit",
	"find every TODO in this repo",
}
