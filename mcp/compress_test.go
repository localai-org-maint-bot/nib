package mcp

import (
	"strings"
	"testing"
)

func TestCompressOutputStripsANSI(t *testing.T) {
	in := "\x1b[32mgreen\x1b[0m \x1b[1;31mred bold\x1b[0m"
	got := compressOutput(in)
	want := "green red bold"
	if got != want {
		t.Fatalf("compressOutput(%q) = %q, want %q", in, got, want)
	}
}

func TestCompressOutputStripsCursorAndOSC(t *testing.T) {
	in := "\x1b[2J\x1b[H\x1b]0;title\x07clean"
	got := compressOutput(in)
	want := "clean"
	if got != want {
		t.Fatalf("compressOutput(%q) = %q, want %q", in, got, want)
	}
}

func TestCollapseRepeats(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no repeats",
			in:   "a\nb\nc",
			want: "a\nb\nc",
		},
		{
			name: "two identical left alone",
			in:   "a\na\nb",
			want: "a\na\nb",
		},
		{
			name: "three collapsed",
			in:   "a\na\na\nb",
			want: "a\n  [2 repeated lines]\nb",
		},
		{
			name: "long run collapsed",
			in:   "x\nx\nx\nx\nx\nx\ny",
			want: "x\n  [5 repeated lines]\ny",
		},
		{
			name: "multiple runs",
			in:   "a\na\na\nb\nb\nb\nb\nc",
			want: "a\n  [2 repeated lines]\nb\n  [3 repeated lines]\nc",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := collapseRepeats(tc.in)
			if got != tc.want {
				t.Fatalf("collapseRepeats(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCompressOutputTailBudget(t *testing.T) {
	big := strings.Repeat("x", bashOutputBudget*2)
	got := compressOutput(big)
	if !strings.HasPrefix(got, "… ") {
		t.Fatal("missing elision marker")
	}
	if !strings.HasSuffix(got, strings.Repeat("x", bashOutputBudget)) {
		t.Fatal("tail not preserved")
	}
}

func TestCompressOutputUnderBudgetUntouched(t *testing.T) {
	small := "hello world"
	got := compressOutput(small)
	if got != small {
		t.Fatalf("compressOutput(%q) = %q, want %q", small, got, small)
	}
}

func TestCompressOutputANSIThenCollapseOrder(t *testing.T) {
	// ANSI codes stripped first, so lines that differ only by colour collapse.
	in := "\x1b[32mfoo\x1b[0m\n\x1b[32mfoo\x1b[0m\n\x1b[32mfoo\x1b[0m\nbar"
	got := compressOutput(in)
	want := "foo\n  [2 repeated lines]\nbar"
	if got != want {
		t.Fatalf("compressOutput = %q, want %q", got, want)
	}
}
