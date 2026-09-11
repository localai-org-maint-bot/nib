package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// fillMessages adds enough short messages that the content exceeds the viewport
// height, so scrolling is meaningful.
func fillMessages(m *Model, n int) {
	for i := 0; i < n; i++ {
		m.messages = append(m.messages, ChatMessage{Role: "user", Content: "history line"})
	}
}

func TestUpdateViewportPreservesScrollWhenNotAtBottom(t *testing.T) {
	m := Model{viewport: viewport.New(40, 4), width: 40}
	fillMessages(&m, 40)
	m.updateViewport()
	if !m.viewport.AtBottom() {
		t.Fatal("precondition: a fresh render should follow to the bottom")
	}

	// User scrolls up to the top.
	m.viewport.SetYOffset(0)
	if m.viewport.AtBottom() {
		t.Fatal("precondition: should not be at bottom after scrolling to top")
	}

	// A re-render (spinner tick, status change, streamed token) must NOT yank the
	// user back to the bottom while they're reading history.
	m.messages = append(m.messages, ChatMessage{Role: "agent", Content: "newly arrived"})
	m.updateViewport()

	if m.viewport.YOffset != 0 {
		t.Fatalf("scroll position not preserved: YOffset = %d, want 0", m.viewport.YOffset)
	}
	if m.viewport.AtBottom() {
		t.Fatal("re-render snapped to bottom while the user was scrolled up")
	}
}

func TestUpdateViewportFollowsWhenAtBottom(t *testing.T) {
	m := Model{viewport: viewport.New(40, 4), width: 40}
	fillMessages(&m, 40)
	m.updateViewport()
	m.viewport.GotoBottom()

	// New content while parked at the bottom should keep following.
	for i := 0; i < 5; i++ {
		m.messages = append(m.messages, ChatMessage{Role: "agent", Content: "streamed"})
	}
	m.updateViewport()

	if !m.viewport.AtBottom() {
		t.Fatal("expected to keep following to the bottom when already at the bottom")
	}
}

// TestResizeRewrapsAndClamps verifies a window resize re-renders at the new
// width and leaves the scroll offset inside the new content bounds.
func TestResizeRewrapsAndClamps(t *testing.T) {
	m := Model{
		viewport: viewport.New(80, 10),
		textarea: textarea.New(),
		width:    80,
		height:   24,
	}
	// One long message so the wrap width visibly changes the line count.
	m.messages = append(m.messages, ChatMessage{
		Role:    "assistant",
		Content: strings.Repeat("wrap me across several lines. ", 40),
	})
	m.updateViewport()
	m.viewport.GotoBottom()
	wide := m.viewport.TotalLineCount()

	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	nm := next.(Model)

	if nm.viewport.Width != 40 {
		t.Fatalf("viewport width = %d, want 40", nm.viewport.Width)
	}
	// The real guard: without the updateViewport() call the viewport still holds
	// content wrapped to the old 80-column width. Halving the width must produce
	// more wrapped lines.
	narrow := nm.viewport.TotalLineCount()
	if narrow <= wide {
		t.Errorf("content not re-wrapped for the narrower width: %d lines at width 40, %d at width 80", narrow, wide)
	}
	if nm.viewport.YOffset > narrow {
		t.Errorf("scroll offset %d stranded past content (%d lines)", nm.viewport.YOffset, narrow)
	}
}
