package render

import "strings"

// SelectList is the shared option picker: ask_user (Phase 3 Task 11),
// /resume (Task 15), and eventually the Ctrl+O job list. It holds no
// bubbletea state — a presenter reads it and renders strings; the model
// mutates it in response to key events.
type SelectList struct {
	Items       []string
	Selected    int
	Checked     []bool // non-nil enables multi-select
	MaxVisible  int    // 0 means unlimited
	MultiSelect bool
}

// Move shifts Selected by delta, wrapping at both ends. It uses
// ((i % n) + n) % n rather than Go's %, which returns a negative result for
// a negative dividend, so a delta of -1 from index 0 lands on the last item
// instead of an invalid negative index. A no-op on an empty list.
func (l *SelectList) Move(delta int) {
	n := len(l.Items)
	if n == 0 {
		return
	}
	l.Selected = (((l.Selected + delta) % n) + n) % n
}

// Page moves the selection by one window's worth of items (MaxVisible),
// wrapping the same way Move does. MaxVisible <= 0 (unlimited) falls back to
// a single-item step.
func (l *SelectList) Page(delta int) {
	step := l.MaxVisible
	if step <= 0 {
		step = 1
	}
	l.Move(delta * step)
}

// Toggle flips the checked state of the current selection. It is a no-op
// unless MultiSelect is enabled (Checked non-nil), and safe on an empty list
// or an out-of-range selection.
func (l *SelectList) Toggle() {
	if !l.MultiSelect || l.Checked == nil {
		return
	}
	if l.Selected < 0 || l.Selected >= len(l.Checked) {
		return
	}
	l.Checked[l.Selected] = !l.Checked[l.Selected]
}

// Answer returns the selected item, or, for multi-select, every checked item
// joined by ", ". It returns "" for an empty list, an out-of-range selection,
// or a multi-select with nothing checked.
func (l *SelectList) Answer() string {
	if l.MultiSelect {
		var picked []string
		for i, checked := range l.Checked {
			if checked && i < len(l.Items) {
				picked = append(picked, l.Items[i])
			}
		}
		return strings.Join(picked, ", ")
	}
	if l.Selected < 0 || l.Selected >= len(l.Items) {
		return ""
	}
	return l.Items[l.Selected]
}

// Window returns the [start, end) slice of Items to display, clamped to
// MaxVisible (0 means unlimited: the whole list) and biased to keep Selected
// off the edges of the window where the list is long enough to allow it.
func (l *SelectList) Window() (start, end int) {
	n := len(l.Items)
	if n == 0 {
		return 0, 0
	}
	max := l.MaxVisible
	if max <= 0 || max >= n {
		return 0, n
	}
	start = l.Selected - max/2
	if start < 0 {
		start = 0
	}
	end = start + max
	if end > n {
		end = n
		start = end - max
	}
	return start, end
}
