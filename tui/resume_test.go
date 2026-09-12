package tui

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

func TestBuildResumeDialog(t *testing.T) {
	list := &render.SelectList{Items: []string{"a · 1m ago · 2 messages", "b · 2h ago · 5 messages"}, Selected: 1}
	d := buildResumeDialog(list)
	if d.Kind != render.DialogResume {
		t.Fatalf("Kind = %v, want DialogResume", d.Kind)
	}
	if d.Title != theme.ResumeTitle {
		t.Errorf("Title = %q, want %q", d.Title, theme.ResumeTitle)
	}
	if len(d.Options) != 2 || d.Options[0].Text != list.Items[0] || d.Options[1].Text != list.Items[1] {
		t.Fatalf("Options mismatch: %+v", d.Options)
	}
	if d.Selected != 1 {
		t.Errorf("Selected = %d, want 1", d.Selected)
	}
	if d.Hint != theme.HelpResume {
		t.Errorf("Hint = %q, want %q", d.Hint, theme.HelpResume)
	}
}

func TestResumeItemsFormatsUntitled(t *testing.T) {
	sessions := []chat.SessionRecord{
		{ID: "a", Title: "fix the bug", Messages: []openai.ChatCompletionMessage{{Role: "user"}, {Role: "assistant"}}},
		{ID: "b", Title: ""},
	}
	items := resumeItems(sessions)
	if !strings.HasPrefix(items[0], "fix the bug · ") || !strings.HasSuffix(items[0], "2 messages") {
		t.Errorf("items[0] = %q", items[0])
	}
	if !strings.HasPrefix(items[1], "(untitled session) · ") {
		t.Errorf("items[1] should fall back to a placeholder title: %q", items[1])
	}
}

func TestStartResumeOpensPickerCwdScoped(t *testing.T) {
	dir := t.TempDir()
	store := chat.NewSessionStore(dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mustSaveTUI(t, store, chat.SessionRecord{ID: "here", Cwd: cwd, Title: "in this dir"})
	mustSaveTUI(t, store, chat.SessionRecord{ID: "elsewhere", Cwd: "/definitely/not/here", Title: "in another dir"})

	m := newTestModel(Model{store: store, textarea: textarea.New(), viewport: viewport.New(80, 20)})
	cmd := m.startResume(false, "")
	if cmd != nil {
		t.Error("startResume should not return a cmd for the picker path")
	}
	if !m.awaitingResume {
		t.Fatal("expected awaitingResume after opening the picker")
	}
	if m.resumeList == nil || len(m.resumeList.Items) != 1 {
		t.Fatalf("expected exactly the cwd-scoped session, got %+v", m.resumeList)
	}
	if len(m.resumeSessions) != 1 || m.resumeSessions[0].ID != "here" {
		t.Fatalf("resumeSessions = %+v, want just the cwd-scoped one", m.resumeSessions)
	}
}

func TestStartResumeAllWidensAcrossCwd(t *testing.T) {
	dir := t.TempDir()
	store := chat.NewSessionStore(dir)
	cwd, _ := os.Getwd()
	mustSaveTUI(t, store, chat.SessionRecord{ID: "here", Cwd: cwd})
	mustSaveTUI(t, store, chat.SessionRecord{ID: "elsewhere", Cwd: "/definitely/not/here"})

	m := newTestModel(Model{store: store, textarea: textarea.New(), viewport: viewport.New(80, 20)})
	m.startResume(true, "")
	if len(m.resumeSessions) != 2 {
		t.Fatalf("--all should list every session regardless of cwd, got %d", len(m.resumeSessions))
	}
}

func TestStartResumeEmptyReportsNotice(t *testing.T) {
	m := newTestModel(Model{store: chat.NewSessionStore(t.TempDir()), textarea: textarea.New(), viewport: viewport.New(80, 20)})
	m.startResume(false, "")
	if m.awaitingResume {
		t.Fatal("an empty store should not open the picker")
	}
	if len(m.messages) != 1 || m.messages[0].Content != theme.ResumeEmpty {
		t.Fatalf("expected the ResumeEmpty notice, got %+v", m.messages)
	}
}

func TestStartResumeWithIDLoadsDirectly(t *testing.T) {
	dir := t.TempDir()
	store := chat.NewSessionStore(dir)
	mustSaveTUI(t, store, chat.SessionRecord{
		ID: "direct", Title: "remembered", Cwd: "/anywhere",
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "remember 41"}, {Role: "assistant", Content: "ok"}},
	})

	m := newTestModel(Model{store: store, ctx: context.Background(), textarea: textarea.New(), viewport: viewport.New(80, 20)})
	cmd := m.startResume(false, "direct")
	if cmd == nil {
		t.Fatal("a direct id resume should return a cmd to re-init the session")
	}
	if m.awaitingResume {
		t.Error("a direct id resume should never open the picker")
	}
	if len(m.cfg.InitialHistory) != 2 {
		t.Fatalf("InitialHistory = %+v, want the 2 seeded messages", m.cfg.InitialHistory)
	}
	if m.sessionID != "direct" || m.sessionTitle != "remembered" {
		t.Errorf("session bookkeeping not adopted from the record: id=%q title=%q", m.sessionID, m.sessionTitle)
	}
}

// TestResumeRestoresTheVisibleTranscript is the I4 regression: applyResume
// seeded cfg.InitialHistory (what the MODEL sees) and appended a "restored N
// messages" notice, but never repopulated m.messages (what the USER sees). A
// resumed session landed on an empty screen claiming it had restored a
// conversation that was nowhere on it.
func TestResumeRestoresTheVisibleTranscript(t *testing.T) {
	dir := t.TempDir()
	store := chat.NewSessionStore(dir)
	mustSaveTUI(t, store, chat.SessionRecord{
		ID: "direct", Title: "remembered", Cwd: "/anywhere",
		Messages: []openai.ChatCompletionMessage{
			{Role: "system", Content: "you are a helpful agent"},
			{Role: "user", Content: "remember 41"},
			{Role: "assistant", Content: "noted: 41"},
			{Role: "assistant", ToolCalls: []openai.ToolCall{{
				Function: openai.FunctionCall{Name: "bash", Arguments: `{"command":"ls -la"}`},
			}}},
			{Role: "tool", Content: "total 0"},
			{Role: "user", Content: "what was it?"},
		},
	})

	m := newTestModel(Model{
		store: store, ctx: context.Background(),
		textarea: textarea.New(), viewport: viewport.New(80, 20),
		width: 80, height: 24,
	})
	m.startResume(false, "direct")

	// What the user sees, not just what the model was handed.
	var roles []string
	for _, msg := range m.messages {
		roles = append(roles, msg.Role)
	}
	want := []string{"user", "assistant", "tool", "user", "agent"} // + the restored notice
	if len(roles) != len(want) {
		t.Fatalf("restored transcript roles = %v, want %v", roles, want)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Fatalf("restored transcript roles = %v, want %v", roles, want)
		}
	}
	if m.messages[0].Content != "remember 41" || m.messages[1].Content != "noted: 41" {
		t.Errorf("restored transcript lost its content: %+v", m.messages[:2])
	}
	if m.messages[2].Name != "bash" {
		t.Errorf("restored tool call lost its name: %+v", m.messages[2])
	}

	// And it reaches the screen, not just the slice.
	m.updateViewport()
	out := m.viewport.View()
	for _, want := range []string{"remember 41", "noted: 41", "what was it?"} {
		if !strings.Contains(out, want) {
			t.Errorf("restored transcript is not on screen: %q missing from %q", want, out)
		}
	}
}

// TestResumeReplacesAnyEarlierTranscript: picking a session mid-conversation
// shows that session, not this one with the other one's messages tacked on.
func TestResumeReplacesAnyEarlierTranscript(t *testing.T) {
	dir := t.TempDir()
	store := chat.NewSessionStore(dir)
	mustSaveTUI(t, store, chat.SessionRecord{
		ID: "direct", Cwd: "/anywhere",
		Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "the stored one"}},
	})

	m := newTestModel(Model{
		store: store, ctx: context.Background(),
		textarea: textarea.New(), viewport: viewport.New(80, 20),
	})
	m = withMessages(m, ChatMessage{Role: "user", Content: "the live one"})
	m.startResume(false, "direct")

	for _, msg := range m.messages {
		if strings.Contains(msg.Content, "the live one") {
			t.Fatalf("the pre-resume transcript survived into the restored one: %+v", m.messages)
		}
	}
}

func TestStartResumeWithBadIDReportsError(t *testing.T) {
	m := newTestModel(Model{store: chat.NewSessionStore(t.TempDir()), textarea: textarea.New(), viewport: viewport.New(80, 20)})
	cmd := m.startResume(false, "nope")
	if cmd != nil {
		t.Error("a load failure should not return a re-init cmd")
	}
	if len(m.messages) != 1 || m.messages[0].Role != "error" {
		t.Fatalf("expected an error notice, got %+v", m.messages)
	}
}

// TestResumeDialogKeyboardNavigation: arrow keys move the picker's selection
// and Enter loads the highlighted session — the same up/down/enter idiom as
// the ask_user dialog (see TestAskDialogKeyboardSelection), reusing
// handleListDialogKey rather than a second copy of the wiring.
func TestResumeDialogKeyboardNavigation(t *testing.T) {
	sessions := []chat.SessionRecord{
		{ID: "alpha", Title: "alpha convo", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "a"}}},
		{ID: "beta", Title: "beta convo", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "b"}}},
	}
	m := newTestModel(Model{
		textarea:       textarea.New(),
		viewport:       viewport.New(80, 20),
		width:          80,
		awaitingResume: true,
		resumeSessions: sessions,
		resumeList:     &render.SelectList{Items: resumeItems(sessions)},
		presenter:      testPresenter(),
	})

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	nm := next.(Model)
	if nm.resumeList.Selected != 1 {
		t.Fatalf("Down did not move the selection: %+v", nm.resumeList)
	}
	if cmd != nil {
		t.Error("navigation should not itself return a cmd")
	}

	next, cmd = nm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	nm = next.(Model)
	if nm.awaitingResume {
		t.Error("awaitingResume still set after picking")
	}
	if nm.sessionID != "beta" {
		t.Errorf("sessionID = %q, want beta (the Down-selected row)", nm.sessionID)
	}
	if len(nm.cfg.InitialHistory) != 1 || nm.cfg.InitialHistory[0].Content != "b" {
		t.Fatalf("InitialHistory not seeded from the selected session: %+v", nm.cfg.InitialHistory)
	}
	if cmd == nil {
		t.Error("picking a session should return the re-init cmd")
	}
}

// TestResumeDialogPaging: pgup/pgdn move the picker a window at a time. The
// spec asked for paging and only up/down was ever wired, so SelectList.Page
// had no caller outside its own unit test — a user with forty stored sessions
// arrowed through them one row at a time.
func TestResumeDialogPaging(t *testing.T) {
	sessions := make([]chat.SessionRecord, 40)
	for i := range sessions {
		sessions[i] = chat.SessionRecord{ID: "s" + strconv.Itoa(i), Title: "convo " + strconv.Itoa(i)}
	}
	const window = 8
	m := newTestModel(Model{
		textarea:       textarea.New(),
		viewport:       viewport.New(80, 20),
		width:          80,
		awaitingResume: true,
		resumeSessions: sessions,
		resumeList:     &render.SelectList{Items: resumeItems(sessions), MaxVisible: window},
		presenter:      testPresenter(),
	})

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	nm := next.(Model)
	if nm.resumeList.Selected != window {
		t.Fatalf("PgDown selected row %d, want %d (one window down)", nm.resumeList.Selected, window)
	}
	if cmd != nil {
		t.Error("paging should not itself return a cmd")
	}

	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	nm = next.(Model)
	if nm.resumeList.Selected != 0 {
		t.Fatalf("PgUp selected row %d, want 0 (back one window)", nm.resumeList.Selected)
	}

	// Clamping, not wrapping: a page off the top stays on the first row.
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	nm = next.(Model)
	if nm.resumeList.Selected != 0 {
		t.Errorf("PgUp at the top wrapped to row %d, want it clamped at 0", nm.resumeList.Selected)
	}
}

// TestResumeDialogEscCancels proves Esc tears down the picker without
// touching cfg — the same contract ask_user's Esc has via resolveAsk("").
func TestResumeDialogEscCancels(t *testing.T) {
	sessions := []chat.SessionRecord{{ID: "alpha"}}
	m := newTestModel(Model{
		textarea:       textarea.New(),
		viewport:       viewport.New(80, 20),
		awaitingResume: true,
		resumeSessions: sessions,
		resumeList:     &render.SelectList{Items: resumeItems(sessions)},
		presenter:      testPresenter(),
	})

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm := next.(Model)
	if nm.awaitingResume || nm.resumeList != nil || nm.resumeSessions != nil {
		t.Fatalf("Esc should clear all picker state: %+v", nm)
	}
	if nm.cfg.InitialHistory != nil {
		t.Error("Esc should not touch cfg.InitialHistory")
	}
	if cmd != nil {
		t.Error("Esc should not return a cmd")
	}
}

// TestResumeDialogSwallowsTyping proves the picker has no free-text fallback
// (unlike ask_user): a character key while it is open does nothing rather
// than landing in the composer.
func TestResumeDialogSwallowsTyping(t *testing.T) {
	sessions := []chat.SessionRecord{{ID: "alpha"}}
	m := newTestModel(Model{
		textarea:       textarea.New(),
		viewport:       viewport.New(80, 20),
		awaitingResume: true,
		resumeSessions: sessions,
		resumeList:     &render.SelectList{Items: resumeItems(sessions)},
		presenter:      testPresenter(),
	})
	m.textarea.Focus()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	nm := next.(Model)
	if strings.TrimSpace(nm.textarea.Value()) != "" {
		t.Errorf("typing while the resume picker is open should be swallowed, got composer = %q", nm.textarea.Value())
	}
	if !nm.awaitingResume {
		t.Error("typing should not close the picker")
	}
}

func mustSaveTUI(t *testing.T, s *chat.SessionStore, rec chat.SessionRecord) {
	t.Helper()
	if err := s.Save(rec); err != nil {
		t.Fatal(err)
	}
}
