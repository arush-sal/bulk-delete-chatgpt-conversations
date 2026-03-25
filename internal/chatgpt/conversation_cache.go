package chatgpt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const conversationCacheEnvVar = "CHATGPT_BULK_CACHE_FILE"

type ConversationVisibility string

const (
	ConversationVisibilityArchived ConversationVisibility = "archived"
	ConversationVisibilityDeleted  ConversationVisibility = "deleted"
)

type ConversationCacheEntry struct {
	Visibility ConversationVisibility `json:"visibility"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

type ConversationCache struct {
	Conversations []Conversation                    `json:"conversations"`
	Hidden        map[string]ConversationCacheEntry `json:"hidden,omitempty"`
	SavedAt       time.Time                         `json:"saved_at"`
}

func ResolveConversationCachePath() (string, error) {
	if override := strings.TrimSpace(os.Getenv(conversationCacheEnvVar)); override != "" {
		return filepath.Clean(override), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}

	return filepath.Join(configDir, "chatgpt-bulk", "conversations.json"), nil
}

func LoadConversationCache() (ConversationCache, string, error) {
	path, err := ResolveConversationCachePath()
	if err != nil {
		return ConversationCache{}, "", err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ConversationCache{}, path, os.ErrNotExist
		}
		return ConversationCache{}, path, fmt.Errorf("read conversation cache: %w", err)
	}

	var cache ConversationCache
	if err := json.Unmarshal(raw, &cache); err != nil {
		return ConversationCache{}, path, fmt.Errorf("decode conversation cache: %w", err)
	}

	cache.Conversations = filterHiddenConversations(cache.Conversations, cache.Hidden)
	if cache.Hidden == nil {
		cache.Hidden = make(map[string]ConversationCacheEntry)
	}

	return cache, path, nil
}

func SaveConversationCache(cache ConversationCache) (string, error) {
	path, err := ResolveConversationCachePath()
	if err != nil {
		return "", err
	}

	cache.Conversations = filterHiddenConversations(cache.Conversations, cache.Hidden)
	if cache.Hidden == nil {
		cache.Hidden = make(map[string]ConversationCacheEntry)
	}
	if cache.SavedAt.IsZero() {
		cache.SavedAt = time.Now().UTC()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create conversation cache dir: %w", err)
	}

	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode conversation cache: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", fmt.Errorf("write conversation cache: %w", err)
	}

	return path, nil
}

func filterHiddenConversations(conversations []Conversation, hidden map[string]ConversationCacheEntry) []Conversation {
	if len(conversations) == 0 {
		return nil
	}
	if len(hidden) == 0 {
		filtered := make([]Conversation, len(conversations))
		copy(filtered, conversations)
		return filtered
	}

	filtered := make([]Conversation, 0, len(conversations))
	for _, conversation := range conversations {
		if _, isHidden := hidden[strings.TrimSpace(conversation.ID)]; isHidden {
			continue
		}
		filtered = append(filtered, conversation)
	}
	return filtered
}
