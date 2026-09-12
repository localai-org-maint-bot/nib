package full

import (
	"strings"
	"testing"

	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// TestMessageRendersRolePrefixes pins each role's chrome. RoleUser/
// RoleAssistant were both a word label before Phase 3 Task 12 ("you"/
// theme.BrandName); this surface now marks them with theme.MsgGutter instead
// (see TestFullUsesGutterNotLabels for the negative half — that the label is
// gone). RoleError is untouched by that task, so it keeps its label check.
func TestMessageRendersRolePrefixes(t *testing.T) {
	p := New()
	cases := []struct {
		name string
		msg  render.Message
		want string
	}{
		{"user", render.Message{Role: render.RoleUser, Content: "hi"}, theme.MsgGutter},
		{"assistant", render.Message{Role: render.RoleAssistant, Content: "hi"}, theme.MsgGutter},
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

// TestFullUsesGutterNotLabels: the full-screen surface has room for a colored
// gutter, which identifies the speaker without spending a word on it.
func TestFullUsesGutterNotLabels(t *testing.T) {
	p := New()
	out := p.Message(render.Message{Role: render.RoleUser, Content: "hello"}, render.RoleNone, 80)
	if strings.Contains(out, "you") {
		t.Error("the full-screen presenter should not print a 'you' label")
	}
	if !strings.Contains(out, theme.MsgGutter) {
		t.Errorf("expected the gutter glyph %q in %q", theme.MsgGutter, out)
	}
}

// TestContinuationLinesCarryTheGutter pins the shape that replaced
// prefix-then-spaces-indent for RoleUser/RoleAssistant: unlike inline (which
// indents continuation lines to the label's width with blank spaces), this
// surface's gutter is a colour bar that must mark EVERY line of the block, or
// wrapped content past the first line would read as unattributed.
func TestContinuationLinesCarryTheGutter(t *testing.T) {
	p := New()
	out := p.Message(render.Message{
		Role:    render.RoleUser,
		Content: strings.Repeat("word ", 40),
	}, render.RoleNone, 30)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped output, got %d line(s)", len(lines))
	}
	if !strings.Contains(lines[1], theme.MsgGutter) {
		t.Errorf("continuation line does not carry the gutter: %q", lines[1])
	}
}

func TestCapsAreFullScreenCaps(t *testing.T) {
	c := New().Caps()
	if !c.AltScreen {
		t.Error("the full-screen presenter must take the alt screen")
	}
	if !c.Mouse {
		t.Error("the full-screen presenter must enable mouse reporting")
	}
	if !c.OverlayDialogs {
		t.Error("the full-screen presenter must overlay dialogs from Frame, not rely on them being baked into body")
	}
}

// TestFooterRowsStyledByKind pins the styling this task's review caught
// regressing once in the inline presenter: FooterJobs/FooterShell must render
// with theme.Meta and lipgloss's Width fill (which pads short lines AND wraps
// ones exceeding the given width — not just padding), while
// FooterLoops/FooterGoal (and the zero-value FooterKindUnset) render with
// theme.Subtle, unfilled. full is a byte-for-byte mirror of inline's footer
// logic, so it carries the same pin. Asserted against the real styles' own
// output, not by reflecting on style objects.
func TestFooterRowsStyledByKind(t *testing.T) {
	p := New()
	const width = 20

	footer := func(kind render.FooterRowKind, text string) string {
		out := p.Footer(render.ViewState{
			Footers: []render.FooterRow{{Text: text, Kind: kind}},
		}, width)
		// Footer always leads with the help line (empty here) then "\n" before
		// the row; strip that to isolate the row's own rendering.
		return strings.TrimPrefix(out, "\n")
	}

	t.Run("jobs gets Meta+Width", func(t *testing.T) {
		text := "jobs: 1 running"
		want := theme.Meta.Width(width).Render(text)
		if got := footer(render.FooterJobs, text); got != want {
			t.Errorf("FooterJobs row = %q, want %q", got, want)
		}
	})

	t.Run("shell gets Meta+Width", func(t *testing.T) {
		text := "shell: 1 running"
		want := theme.Meta.Width(width).Render(text)
		if got := footer(render.FooterShell, text); got != want {
			t.Errorf("FooterShell row = %q, want %q", got, want)
		}
	})

	t.Run("loops gets Subtle, unfilled", func(t *testing.T) {
		text := "1 loop(s): x"
		want := theme.Subtle.Render(text)
		if got := footer(render.FooterLoops, text); got != want {
			t.Errorf("FooterLoops row = %q, want %q", got, want)
		}
	})

	t.Run("goal gets Subtle, unfilled", func(t *testing.T) {
		text := "goal: ship it"
		want := theme.Subtle.Render(text)
		if got := footer(render.FooterGoal, text); got != want {
			t.Errorf("FooterGoal row = %q, want %q", got, want)
		}
	})

	t.Run("unset Kind falls back to the plain default, not Jobs styling", func(t *testing.T) {
		text := "some future row"
		want := theme.Subtle.Render(text)
		if got := footer(render.FooterKindUnset, text); got != want {
			t.Errorf("FooterKindUnset row = %q, want %q (the Subtle default, not Meta+Width)", got, want)
		}
	})

	t.Run("Width wraps an over-long jobs row instead of spilling", func(t *testing.T) {
		long := strings.Repeat("x", width*3)
		want := theme.Meta.Width(width).Render(long)
		got := footer(render.FooterJobs, long)
		if got != want {
			t.Fatalf("over-long FooterJobs row = %q, want %q", got, want)
		}
		lines := strings.Split(got, "\n")
		if len(lines) < 2 {
			t.Fatalf("expected the over-long row to wrap onto multiple lines, got %d: %q", len(lines), got)
		}
	})
}
