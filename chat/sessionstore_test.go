package chat

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func TestSessionStoreRoundTrip(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	rec := SessionRecord{
		ID:      "abc123",
		Title:   "what changed in the last commit?",
		Cwd:     "/home/u/proj",
		Model:   "gpt-4",
		Created: time.Now().Truncate(time.Second),
		Updated: time.Now().Truncate(time.Second),
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != rec.Title || len(got.Messages) != 2 || got.Messages[1].Content != "hi" {
		t.Errorf("round trip lost data: %+v", got)
	}
}

// TestSessionStoreListFiltersByCwd: /resume is cwd-scoped by default so the
// picker stays short and relevant; --all widens it.
func TestSessionStoreListFiltersByCwd(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	mustSave(t, store, SessionRecord{ID: "a", Cwd: "/proj/one", Updated: time.Now()})
	mustSave(t, store, SessionRecord{ID: "b", Cwd: "/proj/two", Updated: time.Now()})

	here, err := store.List("/proj/one")
	if err != nil {
		t.Fatal(err)
	}
	if len(here) != 1 || here[0].ID != "a" {
		t.Errorf("cwd-filtered list = %v, want just session a", here)
	}

	all, err := store.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("unfiltered list has %d sessions, want 2", len(all))
	}
}

// TestSessionStoreListIsNewestFirst: the picker's first row should be the
// session the user most likely wants.
func TestSessionStoreListIsNewestFirst(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	old := time.Now().Add(-time.Hour)
	mustSave(t, store, SessionRecord{ID: "old", Cwd: "/p", Updated: old})
	mustSave(t, store, SessionRecord{ID: "new", Cwd: "/p", Updated: time.Now()})

	got, err := store.List("/p")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "new" {
		t.Errorf("list not newest-first: %v", got)
	}
}

// TestSessionStoreRoundTripPreservesToolCalls proves the message shape most
// likely to lose data silently — an assistant message carrying ToolCalls
// (with a ToolCallID and a function name/arguments) and the matching
// tool-role reply — survives Save/Load intact. /resume's whole promise is
// LOSSLESS restoration; encoding/json round-trips every exported field on
// openai.ChatCompletionMessage (including via its custom Marshal/Unmarshal),
// but that is a structural argument, not evidence, until a test exercises it.
func TestSessionStoreRoundTripPreservesToolCalls(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	rec := SessionRecord{
		ID: "toolcalls1",
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "what's in main.go?"},
			{
				Role: "assistant",
				ToolCalls: []openai.ToolCall{{
					ID:   "call_1",
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"main.go"}`,
					},
				}},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "package main\n"},
		},
	}
	if err := store.Save(rec); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("toolcalls1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("round trip lost messages: got %d, want 3: %+v", len(got.Messages), got.Messages)
	}

	assistant := got.Messages[1]
	if len(assistant.ToolCalls) != 1 {
		t.Fatalf("ToolCalls lost: %+v", assistant)
	}
	call := assistant.ToolCalls[0]
	if call.ID != "call_1" || call.Function.Name != "read_file" || call.Function.Arguments != `{"path":"main.go"}` {
		t.Errorf("ToolCall round trip lost data: %+v", call)
	}

	toolReply := got.Messages[2]
	if toolReply.Role != "tool" || toolReply.ToolCallID != "call_1" || toolReply.Content != "package main\n" {
		t.Errorf("tool-role reply round trip lost data: %+v", toolReply)
	}
}

// TestSessionStoreSaveCreatesPrivateDirectory proves the sessions directory
// is created 0700, not the more permissive 0755 an earlier version used: a
// session FILE being 0600 (os.CreateTemp's default, preserved through
// Save's rename) protects the transcript content, but a world/group-
// readable directory would still let any local user enumerate session ids
// and their timestamps across every project this user has ever run nib in —
// a real privacy leak the directory mode alone controls. Matches the
// existing precedent for sensitive local state: loop/persist.go and
// setup/write.go both use 0700 for the directory holding what they write.
func TestSessionStoreSaveCreatesPrivateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sessions")
	store := NewSessionStore(dir)
	mustSave(t, store, SessionRecord{ID: "a", Cwd: "/p", Updated: time.Now()})

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("sessions directory mode = %o, want 0700", got)
	}
}

func TestSessionStoreLoadMissingIsAnError(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	if _, err := store.Load("nope"); err == nil {
		t.Error("loading an unknown session should error")
	}
}

// TestSessionStoreListSkipsCorruptFile proves one malformed session file does
// not make List (and so /resume) fail outright — it is skipped, and every
// other, well-formed session is still returned.
func TestSessionStoreListSkipsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	mustSave(t, store, SessionRecord{ID: "good", Cwd: "/p", Updated: time.Now()})

	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := store.List("")
	if err != nil {
		t.Fatalf("List returned an error for a corrupt file, want it skipped: %v", err)
	}
	if len(got) != 1 || got[0].ID != "good" {
		t.Errorf("List = %v, want just the one good session", got)
	}
}

// TestSessionStoreSaveLeavesNoTempFile proves Save's atomic write (temp file
// + rename) does not litter the directory with the intermediate file it used
// to get there.
func TestSessionStoreSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	mustSave(t, store, SessionRecord{ID: "abc", Cwd: "/p", Updated: time.Now()})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "abc.json" {
		t.Errorf("directory after Save = %v, want exactly [abc.json]", entries)
	}
}

func mustSave(t *testing.T, s *SessionStore, rec SessionRecord) {
	t.Helper()
	if err := s.Save(rec); err != nil {
		t.Fatal(err)
	}
}
