package mcp

import (
	"fmt"
	"regexp"
	"strings"
)

// bashOutputBudget caps the bytes of stdout or stderr returned by the bash
// tools. Output beyond this is elided from the top, keeping the tail where
// errors and final results usually live. 32 KB ≈ 8 K tokens — generous enough
// for normal command output, tight enough to stop a build log from flooding the
// context.
const bashOutputBudget = 32 * 1024

// ansiRe matches CSI sequences, OSC sequences, and a few common single-char
// escapes (cursor save/restore, charset selection). Enough to clean typical
// terminal colour and cursor output from ls --color, grep --color, etc.
var ansiRe = regexp.MustCompile(
	"\x1b\\[[0-9;?]*[a-zA-Z]" + // CSI: colors, cursor moves, clear
		"|\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)" + // OSC: title, hyperlink
		"|\x1b[()][AB012]" + // Charset designation
		"|\x1b[=>]", // Keypad mode
)

// compressOutput strips ANSI escape codes, collapses runs of repeated lines,
// and applies a tail budget so that a single command cannot flood the context
// with megabytes of output.
func compressOutput(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = collapseRepeats(s)
	if len(s) > bashOutputBudget {
		elided := len(s) - bashOutputBudget
		s = fmt.Sprintf("… %d bytes elided\n%s", elided, s[len(s)-bashOutputBudget:])
	}
	return s
}

// collapseRepeats replaces runs of 3+ identical consecutive lines with the
// first line and a "[N repeated lines]" note. Runs of 2 are left intact —
// a single duplicate is common and not worth a marker.
func collapseRepeats(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= 2 {
		return s
	}
	var out []string
	i := 0
	for i < len(lines) {
		out = append(out, lines[i])
		j := i + 1
		for j < len(lines) && lines[j] == lines[i] {
			j++
		}
		if dups := j - i - 1; dups >= 2 {
			out = append(out, fmt.Sprintf("  [%d repeated lines]", dups))
		} else if dups == 1 {
			out = append(out, lines[i])
		}
		i = j
	}
	return strings.Join(out, "\n")
}
