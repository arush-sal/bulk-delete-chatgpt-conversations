package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
)

func TestUpdateSelectionPageDownKeepsCursorValidWhenFilterHasNoResults(t *testing.T) {
	model := Model{
		phase:      phaseSelect,
		height:     24,
		filterText: "missing",
		conversations: []chatgpt.Conversation{
			{ID: "1", Title: "alpha"},
			{ID: "2", Title: "beta"},
		},
		selected: make(map[string]struct{}),
	}
	model.applyFilterAndSort()

	gotModel, _ := model.updateSelection(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	got, ok := gotModel.(Model)
	if !ok {
		t.Fatalf("updateSelection() returned %T, want tui.Model", gotModel)
	}

	if got.cursor != 0 {
		t.Fatalf("cursor after pgdown with empty filter = %d, want 0", got.cursor)
	}
	if len(got.filtered) != 0 {
		t.Fatalf("filtered count = %d, want 0", len(got.filtered))
	}
}

func TestToggleCurrentIgnoresStaleNegativeCursor(t *testing.T) {
	model := Model{
		cursor: -1,
		filtered: []chatgpt.Conversation{
			{ID: "1", Title: "alpha"},
		},
		selected: make(map[string]struct{}),
	}

	model.toggleCurrent()

	if _, ok := model.selected["1"]; !ok {
		t.Fatal("toggleCurrent() did not recover from a stale negative cursor")
	}
	if model.cursor != 0 {
		t.Fatalf("cursor after toggleCurrent() = %d, want 0", model.cursor)
	}
}
