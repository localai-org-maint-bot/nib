package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

func TestRenderAskAndParse(t *testing.T) {
	req := chat.AskRequest{Question: "Pick one", Options: []string{"alpha", "beta"}}
	list := &render.SelectList{Items: req.Options}
	d := buildAskDialog(req, list)
	if d.Title != "Pick one" || len(d.Options) != 2 || d.Options[0].Text != "alpha" || d.Options[1].Text != "beta" {
		t.Fatalf("ask dialog missing question/options: %+v", d)
	}
	if got := parseAskAnswer("2", req); got != "beta" {
		t.Fatalf("numeric pick: %q", got)
	}
	if got := parseAskAnswer("something else", req); got != "something else" {
		t.Fatalf("free text: %q", got)
	}
	if got := parseAskAnswer("9", req); got != "9" {
		t.Fatalf("out-of-range should be verbatim: %q", got)
	}
	if got := parseAskAnswer("hi", chat.AskRequest{Question: "q"}); got != "hi" {
		t.Fatalf("no-options verbatim: %q", got)
	}
	// Single-select shows a radio marker via the presenter.
	out := testPresenter().Dialog(d, 80)
	if !strings.Contains(out, theme.RadioOff) {
		t.Fatalf("single-select should show a radio marker:\n%s", out)
	}
}

func TestAskMultiSelect(t *testing.T) {
	req := chat.AskRequest{
		Question:    "Pick some",
		Options:     []string{"red", "green", "blue"},
		MultiSelect: true,
	}
	list := &render.SelectList{Items: req.Options, MultiSelect: true, Checked: make([]bool, 3)}
	d := buildAskDialog(req, list)
	out := testPresenter().Dialog(d, 80)
	if !strings.Contains(out, theme.CheckOff) {
		t.Fatalf("multi-select render missing checkbox:\n%s", out)
	}
	if got := parseAskAnswer("1,3", req); got != "red, blue" {
		t.Fatalf("comma indices: %q", got)
	}
	if got := parseAskAnswer("2 3", req); got != "green, blue" {
		t.Fatalf("space indices: %q", got)
	}
	if got := parseAskAnswer("2", req); got != "green" {
		t.Fatalf("single index in multi: %q", got)
	}
	if got := parseAskAnswer("1,9", req); got != "1,9" {
		t.Fatalf("invalid index should be verbatim: %q", got)
	}
	if got := parseAskAnswer("my own answer", req); got != "my own answer" {
		t.Fatalf("free text in multi: %q", got)
	}
}

// TestAskDialogKeyboardSelection: arrow keys move the selection and enter
// answers with the selected option — no number typing required.
func TestAskDialogKeyboardSelection(t *testing.T) {
	respCh := make(chan string, 1)
	m := Model{
		textarea:        textarea.New(),
		viewport:        viewport.New(80, 20),
		width:           80,
		awaitingAsk:     true,
		pendingAsk:      &chat.AskRequest{Question: "which?", Options: []string{"alpha", "beta", "gamma"}},
		askResponseChan: respCh,
		presenter:       testPresenter(),
	}
	m.askList = &render.SelectList{Items: m.pendingAsk.Options}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	next, cmd := next.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	select {
	case got := <-respCh:
		if got != "beta" {
			t.Errorf("answer = %q, want %q", got, "beta")
		}
	default:
		t.Fatal("enter did not answer the question")
	}
	if next.(Model).awaitingAsk {
		t.Error("awaitingAsk still set after answering")
	}
}

// TestAskDialogFreeTextStillWorks preserves the existing escape hatch: typing
// an answer instead of picking one must still send that text.
func TestAskDialogFreeTextStillWorks(t *testing.T) {
	respCh := make(chan string, 1)
	m := Model{
		textarea:        textarea.New(),
		viewport:        viewport.New(80, 20),
		width:           80,
		awaitingAsk:     true,
		pendingAsk:      &chat.AskRequest{Question: "which?", Options: []string{"alpha", "beta"}},
		askResponseChan: respCh,
		presenter:       testPresenter(),
	}
	m.askList = &render.SelectList{Items: m.pendingAsk.Options}
	// bubbles' textarea silently drops key input while unfocused; production
	// code focuses it in the askMsg branch (tui/model.go) before the user can
	// ever type, which this direct Model literal bypasses.
	m.textarea.Focus()

	var cur tea.Model = m
	for _, r := range "custom" {
		cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := cur.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	select {
	case got := <-respCh:
		if got != "custom" {
			t.Errorf("answer = %q, want %q", got, "custom")
		}
	default:
		t.Fatal("free-text answer was not sent")
	}
}

func TestAskDialogMultiSelectTogglesWithSpace(t *testing.T) {
	respCh := make(chan string, 1)
	m := Model{
		textarea:        textarea.New(),
		viewport:        viewport.New(80, 20),
		width:           80,
		awaitingAsk:     true,
		pendingAsk:      &chat.AskRequest{Question: "which?", Options: []string{"alpha", "beta", "gamma"}, MultiSelect: true},
		askResponseChan: respCh,
		presenter:       testPresenter(),
	}
	m.askList = &render.SelectList{Items: m.pendingAsk.Options, MultiSelect: true, Checked: make([]bool, 3)}

	var cur tea.Model = m
	cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeySpace}) // check alpha
	cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	cur, _ = cur.(Model).Update(tea.KeyMsg{Type: tea.KeySpace}) // check gamma
	_, cmd := cur.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}

	select {
	case got := <-respCh:
		if got != "alpha, gamma" {
			t.Errorf("answer = %q, want %q", got, "alpha, gamma")
		}
	default:
		t.Fatal("multi-select answer was not sent")
	}
}
