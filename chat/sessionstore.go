package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// SessionRecord is one recorded conversation: enough to repopulate
// types.Config.InitialHistory and resume losslessly (see
// Session.ExportHistory), plus the metadata the /resume picker lists by
// (Title, Cwd, Updated, message count).
type SessionRecord struct {
	ID       string                         `json:"id"`
	Title    string                         `json:"title"`
	Cwd      string                         `json:"cwd"`
	Model    string                         `json:"model"`
	Created  time.Time                      `json:"created"`
	Updated  time.Time                      `json:"updated"`
	Messages []openai.ChatCompletionMessage `json:"messages"`
}

// SessionStore persists SessionRecords as one JSON file per session under
// Dir. The caller picks Dir; the TUI (tui/model.go's NewModel) and the
// --resume flag (app/app.go's applyResumeFlag) both root it at the per-user
// BaseDir (~/.config/nib/sessions by default) rather than the process's cwd
// the way loop.Registry's loops.json is — cwd-relative would put every
// project's sessions in a different, mutually invisible folder, which
// defeats /resume --all (there would be nothing outside the current folder
// to widen to) and makes the Cwd field below pointless (every session in one
// folder would share the same Cwd by construction). Dir is created on first
// Save; List and Load tolerate it not existing yet.
type SessionStore struct{ Dir string }

// NewSessionStore returns a store rooted at dir. dir is not created until the
// first Save.
func NewSessionStore(dir string) *SessionStore {
	return &SessionStore{Dir: dir}
}

func (s *SessionStore) path(id string) string {
	return filepath.Join(s.Dir, id+".json")
}

// Save writes rec atomically: marshal to a temp file created in the SAME
// directory as the destination, then os.Rename over it. Same-directory
// matters — os.Rename is only atomic within a filesystem, and a temp
// directory elsewhere (e.g. os.TempDir()) could be a different one — and the
// rename itself means a crash mid-write leaves either the old file intact or
// the new one complete, never a truncated one.
func (s *SessionStore) Save(rec SessionRecord) error {
	// 0o700, not 0o755: a session file (0o600, os.CreateTemp's default,
	// preserved through the rename below) protects the transcript CONTENT,
	// but a world/group-readable directory still lets any local user list
	// session ids and their timestamps across every project this user has
	// ever run nib in — a real privacy leak (usage timing/frequency) the
	// directory mode alone controls. Matches the existing precedent for
	// sensitive local state: loop/persist.go and setup/write.go both use
	// 0o700 for the directory holding what they write.
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("sessionstore: create %s: %w", s.Dir, err)
	}
	// MkdirAll does NOT chmod a directory that already exists — a directory
	// left behind 0o755 by an earlier build of this store (before this file
	// used 0o700) would stay world-readable forever otherwise. Chmod is
	// defensive and idempotent, so it runs unconditionally on every Save
	// rather than only after a fresh MkdirAll.
	if err := os.Chmod(s.Dir, 0o700); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("sessionstore: chmod %s: %w", s.Dir, err)
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("sessionstore: marshal %s: %w", rec.ID, err)
	}
	tmp, err := os.CreateTemp(s.Dir, "."+rec.ID+"-*.tmp")
	if err != nil {
		return fmt.Errorf("sessionstore: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Any failure past this point must clean up the temp file rather than
	// leaving it behind — List already ignores ".tmp" files, but there is no
	// reason to litter the directory with dead ones.
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sessionstore: write %s: %w", rec.ID, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("sessionstore: close %s: %w", rec.ID, err)
	}
	if err := os.Rename(tmpPath, s.path(rec.ID)); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("sessionstore: rename %s: %w", rec.ID, err)
	}
	return nil
}

// Load reads one session by id. A missing or corrupt file is an error here
// (unlike List, which skips a corrupt file rather than failing the whole
// listing) — a direct `/resume <id>` naming a bad session has no other
// session to fall back to, so the caller must be told.
func (s *SessionStore) Load(id string) (SessionRecord, error) {
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return SessionRecord{}, fmt.Errorf("sessionstore: load %s: %w", id, err)
	}
	var rec SessionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return SessionRecord{}, fmt.Errorf("sessionstore: parse %s: %w", id, err)
	}
	return rec, nil
}

// List returns every recorded session, newest (Updated) first. cwd == ""
// returns every session regardless of where it was started; a non-empty cwd
// filters to sessions whose stored Cwd matches exactly (so a session started
// in a subdirectory of cwd is excluded — the caller widens with "" to see
// it, matching /resume --all).
//
// A file that fails to read or parse is skipped, not fatal: one corrupt
// session must not make the whole picker unusable. A missing directory
// (nothing recorded yet) returns an empty list, not an error.
func (s *SessionStore) List(cwd string) ([]SessionRecord, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sessionstore: list %s: %w", s.Dir, err)
	}
	var out []SessionRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var rec SessionRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		if cwd != "" && rec.Cwd != cwd {
			continue
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, nil
}
