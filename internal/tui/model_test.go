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

func TestLoadResultUsesCacheThenRefreshesVisibleList(t *testing.T) {
	model := Model{
		phase:    phaseLoading,
		selected: make(map[string]struct{}),
	}

	afterCache, _ := model.Update(loadResultMsg{
		conversations: []chatgpt.Conversation{
			{ID: "cached", Title: "Cached"},
		},
	})

	cachedModel, ok := afterCache.(Model)
	if !ok {
		t.Fatalf("after cache update type = %T, want tui.Model", afterCache)
	}
	if cachedModel.phase != phaseSelect {
		t.Fatalf("phase after cache load = %v, want %v", cachedModel.phase, phaseSelect)
	}
	if len(cachedModel.conversations) != 1 || cachedModel.conversations[0].ID != "cached" {
		t.Fatalf("cached conversations = %#v, want cached conversation", cachedModel.conversations)
	}

	afterRefresh, _ := cachedModel.Update(loadResultMsg{
		refresh: true,
		conversations: []chatgpt.Conversation{
			{ID: "fresh-1", Title: "Fresh 1"},
			{ID: "fresh-2", Title: "Fresh 2"},
		},
	})

	refreshedModel, ok := afterRefresh.(Model)
	if !ok {
		t.Fatalf("after refresh update type = %T, want tui.Model", afterRefresh)
	}
	if got := len(refreshedModel.conversations); got != 2 {
		t.Fatalf("conversation count after refresh = %d, want 2", got)
	}
	if refreshedModel.conversations[0].ID != "fresh-1" || refreshedModel.conversations[1].ID != "fresh-2" {
		t.Fatalf("refreshed conversations = %#v, want refreshed list", refreshedModel.conversations)
	}
}

func TestEmptyCacheLoadWaitsForRefresh(t *testing.T) {
	model := Model{
		phase:    phaseLoading,
		selected: make(map[string]struct{}),
	}

	gotModel, _ := model.Update(loadResultMsg{})
	got, ok := gotModel.(Model)
	if !ok {
		t.Fatalf("Update() returned %T, want tui.Model", gotModel)
	}

	if got.phase != phaseLoading {
		t.Fatalf("phase after empty cache load = %v, want %v", got.phase, phaseLoading)
	}
	if len(got.conversations) != 0 {
		t.Fatalf("conversations after empty cache load = %#v, want empty", got.conversations)
	}
}
