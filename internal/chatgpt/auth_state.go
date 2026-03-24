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

const authFileEnvVar = "CHATGPT_BULK_AUTH_FILE"

type AuthState struct {
	SessionToken string    `json:"session_token"`
	CSRFToken    string    `json:"csrf_token,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	UserEmail    string    `json:"user_email,omitempty"`
	SavedAt      time.Time `json:"saved_at"`
	Source       string    `json:"source,omitempty"`
}

type AuthSource string

const (
	AuthSourceNone   AuthSource = "none"
	AuthSourceStored AuthSource = "stored"
)

type ResolvedAuth struct {
	State  AuthState
	Source AuthSource
	Path   string
}

func ResolveAuthStatePath() (string, error) {
	if override := strings.TrimSpace(os.Getenv(authFileEnvVar)); override != "" {
		return filepath.Clean(override), nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}

	return filepath.Join(configDir, "chatgpt-bulk", "auth.json"), nil
}

func LoadAuthState() (AuthState, string, error) {
	path, err := ResolveAuthStatePath()
	if err != nil {
		return AuthState{}, "", err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AuthState{}, path, os.ErrNotExist
		}
		return AuthState{}, path, fmt.Errorf("read auth state: %w", err)
	}

	var state AuthState
	if err := json.Unmarshal(raw, &state); err != nil {
		return AuthState{}, path, fmt.Errorf("decode auth state: %w", err)
	}
	state.SessionToken = strings.TrimSpace(state.SessionToken)
	state.CSRFToken = strings.TrimSpace(state.CSRFToken)
	state.AccessToken = strings.TrimSpace(state.AccessToken)
	state.UserEmail = strings.TrimSpace(state.UserEmail)
	if state.SessionToken == "" {
		return AuthState{}, path, errors.New("stored auth is missing a session token")
	}

	return state, path, nil
}

func SaveAuthState(state AuthState) (string, error) {
	path, err := ResolveAuthStatePath()
	if err != nil {
		return "", err
	}

	state.SessionToken = strings.TrimSpace(state.SessionToken)
	state.CSRFToken = strings.TrimSpace(state.CSRFToken)
	state.AccessToken = strings.TrimSpace(state.AccessToken)
	state.UserEmail = strings.TrimSpace(state.UserEmail)
	if state.SessionToken == "" {
		return "", errors.New("cannot save auth state without a session token")
	}
	if state.SavedAt.IsZero() {
		state.SavedAt = time.Now().UTC()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create auth state dir: %w", err)
	}

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode auth state: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", fmt.Errorf("write auth state: %w", err)
	}

	return path, nil
}

func RemoveAuthState() (string, bool, error) {
	path, err := ResolveAuthStatePath()
	if err != nil {
		return "", false, err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, false, nil
		}
		return path, false, fmt.Errorf("remove auth state: %w", err)
	}
	return path, true, nil
}

func ResolveAuth() (ResolvedAuth, error) {
	if stored, storedPath, err := LoadAuthState(); err == nil {
		return ResolvedAuth{State: stored, Source: AuthSourceStored, Path: storedPath}, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return ResolvedAuth{}, err
	}

	path, err := ResolveAuthStatePath()
	if err != nil {
		return ResolvedAuth{}, err
	}

	return ResolvedAuth{Source: AuthSourceNone, Path: path}, nil
}

func MaskToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "Unavailable"
	}
	if len(token) <= 16 {
		return token
	}
	return token[:8] + "..." + token[len(token)-6:]
}
