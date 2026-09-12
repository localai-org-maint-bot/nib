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
