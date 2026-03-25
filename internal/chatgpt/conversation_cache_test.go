package chatgpt

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestResolveConversationCachePathOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "custom-conversations.json")
	t.Setenv(conversationCacheEnvVar, override)

	path, err := ResolveConversationCachePath()
	if err != nil {
		t.Fatalf("ResolveConversationCachePath() error = %v", err)
	}
	if path != override {
		t.Fatalf("ResolveConversationCachePath() = %q, want %q", path, override)
	}
}

func TestSaveLoadConversationCacheFiltersHiddenEntries(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "conversations.json")
	t.Setenv(conversationCacheEnvVar, cachePath)

	savedAt := time.Date(2026, time.March, 25, 8, 0, 0, 0, time.UTC)
	hiddenAt := time.Date(2026, time.March, 25, 7, 0, 0, 0, time.UTC)

	cache := ConversationCache{
		Conversations: []Conversation{
			{ID: "visible", Title: "Visible chat"},
			{ID: "hidden", Title: "Hidden chat"},
		},
		Hidden: map[string]ConversationCacheEntry{
			"hidden": {
				Visibility: ConversationVisibilityArchived,
				UpdatedAt:  hiddenAt,
			},
		},
		SavedAt: savedAt,
	}

	path, err := SaveConversationCache(cache)
	if err != nil {
		t.Fatalf("SaveConversationCache() error = %v", err)
	}
	if path != cachePath {
		t.Fatalf("SaveConversationCache() path = %q, want %q", path, cachePath)
	}

	loaded, loadedPath, err := LoadConversationCache()
	if err != nil {
		t.Fatalf("LoadConversationCache() error = %v", err)
	}
	if loadedPath != cachePath {
		t.Fatalf("LoadConversationCache() path = %q, want %q", loadedPath, cachePath)
	}
	if len(loaded.Conversations) != 1 || loaded.Conversations[0].ID != "visible" {
		t.Fatalf("LoadConversationCache() conversations = %#v, want only visible conversation", loaded.Conversations)
	}
	if entry, ok := loaded.Hidden["hidden"]; !ok {
		t.Fatal("LoadConversationCache() missing hidden conversation entry")
	} else if entry.Visibility != ConversationVisibilityArchived || !entry.UpdatedAt.Equal(hiddenAt) {
		t.Fatalf("LoadConversationCache() hidden entry = %#v, want archived at %v", entry, hiddenAt)
	}
	if !loaded.SavedAt.Equal(savedAt) {
		t.Fatalf("LoadConversationCache() savedAt = %v, want %v", loaded.SavedAt, savedAt)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(cachePath)
		if err != nil {
			t.Fatalf("os.Stat(%q) error = %v", cachePath, err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("conversation cache mode = %#o, want %#o", mode, 0o600)
		}
	}
}
