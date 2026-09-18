package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mudler/nib/chat"
	"github.com/mudler/nib/theme"
	"github.com/mudler/nib/tui/render"
)

// modelPickerMaxVisible is the most model rows the dialog shows at once.
// The list scrolls within this window, biased to keep the selection visible.
const modelPickerMaxVisible = 8

type modelPicker struct {
	active    bool
	loading   bool
	requestID uint64
	// target is the provider being switched to (a /login pick), or nil for
	// the session's current provider (/model).
	target *chat.ProviderEntry
	// typed means no model list is available for target: the query itself
	// is the model name Enter uses.
	typed    bool
	listErr  string
	all      []string
	matches  []string
	query    string
	selected int
	offset   int
}

func (p *modelPicker) open(requestID uint64) {
	*p = modelPicker{active: true, loading: true, requestID: requestID}
}

func (p *modelPicker) close() {
	*p = modelPicker{}
}

func (p *modelPicker) setModels(models []string, current string) {
	p.loading = false
	p.all = append([]string(nil), models...)
	p.matches = append([]string(nil), models...)
	p.selected = 0
	p.offset = 0
	for i, model := range p.matches {
		if model == current {
			p.selected = i
			break
		}
	}
	p.scrollSelectionIntoView(modelPickerMaxVisible)
}

func (p *modelPicker) appendQuery(text string) {
	p.query += text
	p.filter()
}

func (p *modelPicker) backspace() {
	runes := []rune(p.query)
	if len(runes) == 0 {
		return
	}
	p.query = string(runes[:len(runes)-1])
	p.filter()
}

func (p *modelPicker) filter() {
	p.matches = p.matches[:0]
	query := strings.ToLower(p.query)
	for _, model := range p.all {
		if strings.Contains(strings.ToLower(model), query) {
			p.matches = append(p.matches, model)
		}
	}
	p.selected = 0
	p.offset = 0
	p.scrollSelectionIntoView(modelPickerMaxVisible)
}

func (p *modelPicker) move(delta int) {
	if len(p.matches) == 0 {
		p.selected = 0
		p.offset = 0
		return
	}
	p.selected += delta
	if p.selected < 0 {
		p.selected = 0
	}
	if p.selected >= len(p.matches) {
		p.selected = len(p.matches) - 1
	}
	p.scrollSelectionIntoView(modelPickerMaxVisible)
}

func (p *modelPicker) scrollSelectionIntoView(visible int) {
	if visible < 1 {
		visible = 1
	}
	if p.selected < p.offset {
		p.offset = p.selected
	}
	if p.selected >= p.offset+visible {
		p.offset = p.selected - visible + 1
	}
	if p.offset < 0 {
		p.offset = 0
	}
}

func (p modelPicker) choice() (string, bool) {
	if p.selected < 0 || p.selected >= len(p.matches) {
		// A provider being switched to may serve models its list omits, and
		// some have no list at all: Enter takes the typed name as-is. /model
		// keeps refusing unlisted names (see Session.SwitchModel).
		if (p.typed || p.target != nil) && strings.TrimSpace(p.query) != "" {
			return strings.TrimSpace(p.query), true
		}
		return "", false
	}
	return p.matches[p.selected], true
}

// window returns the [start, end) slice of matches currently visible in the
// dialog, clamped to modelPickerMaxVisible and offset.
func (p modelPicker) window() (start, end int) {
	n := len(p.matches)
	if n == 0 {
		return 0, 0
	}
	start = p.offset
	if start < 0 {
		start = 0
	}
	if start >= n {
		start = n - 1
	}
	end = start + modelPickerMaxVisible
	if end > n {
		end = n
	}
	return start, end
}

// buildModelPickerDialog produces the render.Dialog for the model picker,
// mirroring buildResumeDialog: the search query is the Title, the visible
// window of filtered matches are the Options, and the key hint (or a
// loading/empty/no-match message) is the Hint.
func (m Model) buildModelPickerDialog() render.Dialog {
	p := m.modelPicker
	current := ""
	if m.session != nil && p.target == nil {
		current = m.session.Model()
	}

	search := theme.ModelPickerSearchLabel
	if p.typed {
		search = theme.ModelPickerNameLabel
	}
	if p.target != nil {
		search = p.target.Name + " model " + search
	}
	if p.query != "" {
		search += " " + p.query
	}

	d := render.Dialog{
		Kind:  render.DialogModelPicker,
		Title: search,
		Hint:  theme.ModelPickerKeyHint,
	}

	switch {
	case p.loading:
		d.Hint = theme.ModelPickerLoading
	case p.typed:
		d.Hint = theme.ModelPickerTypeName
		if p.listErr != "" {
			d.Hint = p.listErr + " · " + d.Hint
		}
	case len(p.all) == 0:
		d.Hint = theme.ModelPickerEmpty
	case len(p.matches) == 0 && p.target != nil:
		d.Hint = theme.ModelPickerNoMatches + " " + theme.ModelPickerUseTyped
	case len(p.matches) == 0:
		d.Hint = theme.ModelPickerNoMatches
	default:
		start, end := p.window()
		for i := start; i < end; i++ {
			text := p.matches[i]
			if p.matches[i] == current {
				text += " (current)"
			}
			d.Options = append(d.Options, render.DialogOption{Text: text})
		}
		d.Selected = p.selected - start
	}

	return d
}

func clipModelPickerLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= width {
		return line
	}
	if width == 1 {
		return "…"
	}

	var b strings.Builder
	used := 0
	for _, r := range line {
		runeWidth := lipgloss.Width(string(r))
		if used+runeWidth > width-1 {
			break
		}
		b.WriteRune(r)
		used += runeWidth
	}
	b.WriteRune('…')
	return b.String()
}
