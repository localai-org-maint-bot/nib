package inline

import (
	"strings"
	"testing"

	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

func TestMessageRendersRolePrefixes(t *testing.T) {
	p := New()
	cases := []struct {
		name string
		msg  render.Message
		want string
	}{
		{"user", render.Message{Role: render.RoleUser, Content: "hi"}, "you"},
		{"assistant", render.Message{Role: render.RoleAssistant, Content: "hi"}, theme.BrandName},
		{"error", render.Message{Role: render.RoleError, Content: "boom"}, theme.Cross},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := p.Message(c.msg, render.RoleNone, 80)
			if !strings.Contains(got, c.want) {
				t.Errorf("Message(%v) = %q, want it to contain %q", c.msg, got, c.want)
			}
		})
	}
}

func TestContinuationLinesAreIndentedToPrefixWidth(t *testing.T) {
	p := New()
	out := p.Message(render.Message{
		Role:    render.RoleUser,
		Content: strings.Repeat("word ", 40),
	}, render.RoleNone, 30)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped output, got %d line(s)", len(lines))
	}
	if !strings.HasPrefix(lines[1], "      ") {
		t.Errorf("continuation line not indented to the prefix width: %q", lines[1])
	}
}

func TestCapsAreInlineWidgetCaps(t *testing.T) {
	c := New().Caps()
	if c.AltScreen {
		t.Error("the inline widget must not take the alt screen")
	}
	if c.Mouse {
		t.Error("the inline widget does not enable mouse reporting")
	}
}
