package render

import "testing"

func TestSelectListMoveWraps(t *testing.T) {
	cases := []struct {
		name  string
		start int
		delta int
		want  int
	}{
		{"down", 0, 1, 1},
		{"up", 1, -1, 0},
		{"wraps past the end", 2, 1, 0},
		{"wraps before the start", 0, -1, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := &SelectList{Items: []string{"a", "b", "c"}, Selected: c.start}
			l.Move(c.delta)
			if l.Selected != c.want {
				t.Errorf("Selected = %d, want %d", l.Selected, c.want)
			}
		})
	}
}

func TestSelectListEmptyIsSafe(t *testing.T) {
	l := &SelectList{}
	l.Move(1)
	l.Toggle()
	if got := l.Answer(); got != "" {
		t.Errorf("Answer() on an empty list = %q, want empty", got)
	}
}

func TestSelectListMultiSelectAnswer(t *testing.T) {
	l := &SelectList{Items: []string{"a", "b", "c"}, MultiSelect: true, Checked: make([]bool, 3)}
	l.Toggle() // check "a"
	l.Move(2)  // to "c"
	l.Toggle() // check "c"
	if got, want := l.Answer(), "a, c"; got != want {
		t.Errorf("Answer() = %q, want %q", got, want)
	}
}

func TestSelectListWindowContainsSelection(t *testing.T) {
	l := &SelectList{Items: []string{"a", "b", "c", "d", "e", "f"}, MaxVisible: 3, Selected: 5}
	start, end := l.Window()
	if l.Selected < start || l.Selected >= end {
		t.Errorf("window [%d,%d) does not contain selection %d", start, end, l.Selected)
	}
	if end-start != 3 {
		t.Errorf("window size = %d, want 3", end-start)
	}
}

func TestSelectListWindowMaxVisibleExceedsItemCount(t *testing.T) {
	l := &SelectList{Items: []string{"a", "b", "c"}, MaxVisible: 10}
	start, end := l.Window()
	if start != 0 || end != 3 {
		t.Errorf("Window() = (%d, %d), want (0, 3)", start, end)
	}
}

func TestSelectListWindowUnlimited(t *testing.T) {
	l := &SelectList{Items: []string{"a", "b", "c", "d"}, MaxVisible: 0, Selected: 2}
	start, end := l.Window()
	if start != 0 || end != 4 {
		t.Errorf("Window() = (%d, %d), want (0, 4)", start, end)
	}
}

func TestSelectListWindowEmptyIsSafe(t *testing.T) {
	l := &SelectList{}
	start, end := l.Window()
	if start != 0 || end != 0 {
		t.Errorf("Window() on empty list = (%d, %d), want (0, 0)", start, end)
	}
}

func TestSelectListMoveDeltaLargerThanLength(t *testing.T) {
	l := &SelectList{Items: []string{"a", "b", "c"}, Selected: 0}
	l.Move(7) // 7 mod 3 == 1
	if l.Selected != 1 {
		t.Errorf("Selected = %d, want 1", l.Selected)
	}
	l.Selected = 0
	l.Move(-7) // wraps the same way a -1 would
	if l.Selected != 2 {
		t.Errorf("Selected = %d, want 2", l.Selected)
	}
}

func TestSelectListPageMovesByMaxVisible(t *testing.T) {
	l := &SelectList{Items: []string{"a", "b", "c", "d", "e", "f"}, MaxVisible: 2, Selected: 0}
	l.Page(1)
	if l.Selected != 2 {
		t.Errorf("Selected = %d, want 2", l.Selected)
	}
}

func TestSelectListMultiSelectAnswerNothingChecked(t *testing.T) {
	l := &SelectList{Items: []string{"a", "b", "c"}, MultiSelect: true, Checked: make([]bool, 3)}
	if got := l.Answer(); got != "" {
		t.Errorf("Answer() = %q, want empty", got)
	}
}

func TestSelectListToggleNoopWithoutMultiSelect(t *testing.T) {
	l := &SelectList{Items: []string{"a", "b"}}
	l.Toggle() // MultiSelect is false, Checked is nil: must not panic
}
