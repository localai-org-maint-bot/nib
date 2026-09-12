package render_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mudler/nib/tui/render"
	"github.com/mudler/nib/tui/render/full"
	"github.com/mudler/nib/tui/render/inline"
)

// presenters is every implementation. Adding one here is how a new surface
// proves it behaves like the others.
func presenters() map[string]render.Presenter {
	return map[string]render.Presenter{
		"inline": inline.New(),
		"full":   full.New(),
	}
}

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes SGR escape sequences so width/content checks measure only
// visible runes.
func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// TestAllPresentersRenderMessageContent: chrome may differ, the user's words may not.
func TestAllPresentersRenderMessageContent(t *testing.T) {
	for name, p := range presenters() {
		t.Run(name, func(t *testing.T) {
			out := p.Message(render.Message{Role: render.RoleUser, Content: "distinctive content"}, render.RoleNone, 80)
			if !strings.Contains(out, "distinctive content") {
				t.Errorf("%s dropped the message content: %q", name, out)
			}
		})
	}
}

// TestAllPresentersRespectWidth: no presenter may emit a line wider than the
// budget it was given, or the inline widget corrupts the surrounding shell.
func TestAllPresentersRespectWidth(t *testing.T) {
	const w = 40
	for name, p := range presenters() {
		t.Run(name, func(t *testing.T) {
			// RoleUser deliberately: Message wraps user content itself. Assistant
			// and agent content arrives pre-rendered (glamour is width-cached
			// state owned by the model), so it is not the presenter's to wrap.
			out := p.Message(render.Message{
				Role:    render.RoleUser,
				Content: strings.Repeat("overflowing ", 30),
			}, render.RoleNone, w)
			for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
				if got := len([]rune(stripANSI(line))); got > w {
					t.Errorf("%s line %d is %d cells wide, budget %d: %q", name, i, got, w, line)
				}
			}
		})
	}
}

// NOTE: selection-marking conformance is deliberately NOT asserted here.
// At this task `Dialog` is the legacy ask block moved verbatim — static text
// that ignores `Selected`. Selection rendering arrives in Phase 3 Task 11, and
// the conformance test for it lands there with the behaviour it guards.
