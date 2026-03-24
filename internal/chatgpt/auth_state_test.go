package chatgpt

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestResolveAuthStatePathOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), "custom-auth.json")
	t.Setenv(authFileEnvVar, override)

	path, err := ResolveAuthStatePath()
	if err != nil {
		t.Fatalf("ResolveAuthStatePath() error = %v", err)
	}
	if path != override {
		t.Fatalf("ResolveAuthStatePath() = %q, want %q", path, override)
	}
}

func TestSaveLoadAndRemoveAuthState(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv(authFileEnvVar, authPath)

	savedAt := time.Date(2026, time.March, 24, 10, 30, 0, 0, time.UTC)
	state := AuthState{
		SessionToken: "session-token-value",
		CSRFToken:    "csrf-token-value",
		AccessToken:  "access-token-value",
		UserEmail:    "user@example.com",
		SavedAt:      savedAt,
		Source:       "browser",
	}

	path, err := SaveAuthState(state)
	if err != nil {
		t.Fatalf("SaveAuthState() error = %v", err)
	}
	if path != authPath {
		t.Fatalf("SaveAuthState() path = %q, want %q", path, authPath)
	}

	loaded, loadedPath, err := LoadAuthState()
	if err != nil {
		t.Fatalf("LoadAuthState() error = %v", err)
	}
	if loadedPath != authPath {
		t.Fatalf("LoadAuthState() path = %q, want %q", loadedPath, authPath)
	}
	if loaded != state {
		t.Fatalf("LoadAuthState() = %#v, want %#v", loaded, state)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(authPath)
		if err != nil {
			t.Fatalf("os.Stat(%q) error = %v", authPath, err)
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("auth file mode = %#o, want %#o", mode, 0o600)
		}
	}

	removedPath, removed, err := RemoveAuthState()
	if err != nil {
		t.Fatalf("RemoveAuthState() error = %v", err)
	}
	if removedPath != authPath {
		t.Fatalf("RemoveAuthState() path = %q, want %q", removedPath, authPath)
	}
	if !removed {
		t.Fatalf("RemoveAuthState() removed = false, want true")
	}
	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatalf("auth file still exists after RemoveAuthState(): stat err = %v", err)
	}
}

func TestResolveAuthPrefersStoredOverEnv(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv(authFileEnvVar, authPath)
	t.Setenv("CHATGPT_SESSION_TOKEN", "env-session")
	t.Setenv("CHATGPT_CSRF_TOKEN", "env-csrf")

	_, err := SaveAuthState(AuthState{
		SessionToken: "stored-session",
		CSRFToken:    "stored-csrf",
		AccessToken:  "stored-access",
		UserEmail:    "stored@example.com",
		SavedAt:      time.Date(2026, time.March, 24, 11, 0, 0, 0, time.UTC),
		Source:       "browser",
	})
	if err != nil {
		t.Fatalf("SaveAuthState() error = %v", err)
	}

	resolved, err := ResolveAuth(Config{})
	if err != nil {
		t.Fatalf("ResolveAuth() error = %v", err)
	}
	if resolved.Source != AuthSourceStored {
		t.Fatalf("ResolveAuth() source = %q, want %q", resolved.Source, AuthSourceStored)
	}
	if resolved.State.SessionToken != "stored-session" {
		t.Fatalf("ResolveAuth() session token = %q, want stored value", resolved.State.SessionToken)
	}
}

func TestResolveAuthFallsBackToEnv(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv(authFileEnvVar, authPath)
	t.Setenv("CHATGPT_SESSION_TOKEN", "env-session")
	t.Setenv("CHATGPT_CSRF_TOKEN", "env-csrf")

	resolved, err := ResolveAuth(Config{})
	if err != nil {
		t.Fatalf("ResolveAuth() error = %v", err)
	}
	if resolved.Source != AuthSourceEnv {
		t.Fatalf("ResolveAuth() source = %q, want %q", resolved.Source, AuthSourceEnv)
	}
	if resolved.State.SessionToken != "env-session" {
		t.Fatalf("ResolveAuth() session token = %q, want env value", resolved.State.SessionToken)
	}
}

func TestResolveAuthPrefersExplicitConfigOverStoredAndEnv(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv(authFileEnvVar, authPath)
	t.Setenv("CHATGPT_SESSION_TOKEN", "env-session")
	t.Setenv("CHATGPT_CSRF_TOKEN", "env-csrf")

	_, err := SaveAuthState(AuthState{
		SessionToken: "stored-session",
		CSRFToken:    "stored-csrf",
		SavedAt:      time.Date(2026, time.March, 24, 11, 30, 0, 0, time.UTC),
		Source:       "browser",
	})
	if err != nil {
		t.Fatalf("SaveAuthState() error = %v", err)
	}

	resolved, err := ResolveAuth(Config{
		SessionToken: "config-session",
		CSRFToken:    "config-csrf",
	})
	if err != nil {
		t.Fatalf("ResolveAuth() error = %v", err)
	}
	if resolved.Source != AuthSourceConfig {
		t.Fatalf("ResolveAuth() source = %q, want %q", resolved.Source, AuthSourceConfig)
	}
	if resolved.State.SessionToken != "config-session" {
		t.Fatalf("ResolveAuth() session token = %q, want config value", resolved.State.SessionToken)
	}
	if resolved.State.CSRFToken != "config-csrf" {
		t.Fatalf("ResolveAuth() csrf token = %q, want config value", resolved.State.CSRFToken)
	}
}

func TestResolveAuthIgnoresWhitespaceConfigAndFallsBackToEnv(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv(authFileEnvVar, authPath)
	t.Setenv("CHATGPT_SESSION_TOKEN", "env-session")
	t.Setenv("CHATGPT_CSRF_TOKEN", "env-csrf")

	resolved, err := ResolveAuth(Config{
		SessionToken: " \t\n ",
		CSRFToken:    " \t\n ",
	})
	if err != nil {
		t.Fatalf("ResolveAuth() error = %v", err)
	}
	if resolved.Source != AuthSourceEnv {
		t.Fatalf("ResolveAuth() source = %q, want %q", resolved.Source, AuthSourceEnv)
	}
	if resolved.State.SessionToken != "env-session" {
		t.Fatalf("ResolveAuth() session token = %q, want env value after trimming whitespace config", resolved.State.SessionToken)
	}
	if resolved.State.CSRFToken != "env-csrf" {
		t.Fatalf("ResolveAuth() csrf token = %q, want env value after trimming whitespace config", resolved.State.CSRFToken)
	}
}

func TestResolveAuthReturnsStoredDecodeError(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv(authFileEnvVar, authPath)

	if err := os.WriteFile(authPath, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", authPath, err)
	}

	_, err := ResolveAuth(Config{})
	if err == nil {
		t.Fatalf("ResolveAuth() error = nil, want decode failure")
	}
	if got := err.Error(); got == "" || got == os.ErrNotExist.Error() {
		t.Fatalf("ResolveAuth() error = %v, want decode failure", err)
	}
}
