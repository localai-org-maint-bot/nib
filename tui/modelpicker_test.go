package tui

import (
	"reflect"
	"testing"
)

func TestModelPickerOpenAndCloseResetState(t *testing.T) {
	p := modelPicker{active: true, all: []string{"old"}, query: "old", selected: 3, offset: 2}
	p.open(17)
	if !p.active || !p.loading || p.requestID != 17 {
		t.Fatalf("open state = %+v", p)
	}
	if len(p.all) != 0 || len(p.matches) != 0 || p.query != "" || p.selected != 0 || p.offset != 0 {
		t.Fatalf("open did not reset prior state: %+v", p)
	}

	p.close()
	if !reflect.DeepEqual(p, modelPicker{}) {
		t.Fatalf("close state = %+v, want zero value", p)
	}
}

func TestModelPickerSetModelsPreservesOrderAndSelectsCurrent(t *testing.T) {
	p := modelPicker{active: true, loading: true}
	p.setModels([]string{"zeta", "Alpha", "beta", "alpine"}, "beta")
	if p.loading {
		t.Fatal("picker remained loading after models arrived")
	}
	if want := []string{"zeta", "Alpha", "beta", "alpine"}; !reflect.DeepEqual(p.matches, want) {
		t.Fatalf("matches = %v, want endpoint order %v", p.matches, want)
	}
	if p.selected != 2 || p.offset != 0 {
		t.Fatalf("selection = %d, offset = %d; want current at 2 and offset 0", p.selected, p.offset)
	}
	if got, ok := p.choice(); !ok || got != "beta" {
		t.Fatalf("choice = %q, %v; want beta, true", got, ok)
	}
}

func TestModelPickerSetModelsFallsBackToFirstResult(t *testing.T) {
	p := modelPicker{active: true, loading: true}
	p.setModels([]string{"first", "second"}, "missing")
	if p.selected != 0 || p.offset != 0 {
		t.Fatalf("selection = %d, offset = %d; want first result", p.selected, p.offset)
	}
}

func TestModelPickerQueryFiltersCaseInsensitiveSubstringInEndpointOrder(t *testing.T) {
	p := modelPicker{all: []string{"Zulu", "ALPHA-large", "beta", "small-alpha"}, selected: 3, offset: 2}
	p.appendQuery("aLpHa")
	if want := []string{"ALPHA-large", "small-alpha"}; !reflect.DeepEqual(p.matches, want) {
		t.Fatalf("matches = %v, want %v", p.matches, want)
	}
	if p.selected != 0 || p.offset != 0 {
		t.Fatalf("query change selection = %d, offset = %d; want reset", p.selected, p.offset)
	}
}

func TestModelPickerBackspaceRemovesOneRuneAndRefilters(t *testing.T) {
	p := modelPicker{all: []string{"café", "cafeteria", "tea"}}
	p.appendQuery("fé")
	p.backspace()
	if p.query != "f" {
		t.Fatalf("query = %q, want rune-safe removal to f", p.query)
	}
	if want := []string{"café", "cafeteria"}; !reflect.DeepEqual(p.matches, want) {
		t.Fatalf("matches = %v, want %v", p.matches, want)
	}
	p.backspace()
	p.backspace()
	if p.query != "" {
		t.Fatalf("backspace past empty query = %q", p.query)
	}
}

func TestModelPickerMoveIsBoundedAndScrollsSelectionIntoView(t *testing.T) {
	p := modelPicker{matches: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}}
	p.move(-10)
	if p.selected != 0 || p.offset != 0 {
		t.Fatalf("move above start = selected %d offset %d", p.selected, p.offset)
	}
	p.move(10)
	if p.selected != 10 || p.offset != 3 {
		t.Fatalf("move to 11th = selected %d offset %d; want 10, 3", p.selected, p.offset)
	}
	p.move(20)
	if p.selected != 11 || p.offset != 4 {
		t.Fatalf("move beyond end = selected %d offset %d; want 11, 4", p.selected, p.offset)
	}
	p.move(-2)
	if p.selected != 9 || p.offset != 4 {
		t.Fatalf("move within window = selected %d offset %d; want 9, 4", p.selected, p.offset)
	}
}

func TestModelPickerChoiceRejectsEmptyOrInvalidSelection(t *testing.T) {
	for _, p := range []modelPicker{{}, {matches: []string{"one"}, selected: -1}, {matches: []string{"one"}, selected: 1}} {
		if got, ok := p.choice(); ok || got != "" {
			t.Fatalf("choice for %+v = %q, %v; want empty, false", p, got, ok)
		}
	}
}
