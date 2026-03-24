package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
)

type stubAuthClient struct {
	authenticate func(context.Context) (chatgpt.AuthState, error)
	closeCalls   int
}

func (s *stubAuthClient) Authenticate(ctx context.Context) (chatgpt.AuthState, error) {
	if s.authenticate != nil {
		return s.authenticate(ctx)
	}
	return chatgpt.AuthState{}, nil
}

func (s *stubAuthClient) Close() {
	s.closeCalls++
}

func withMainTestHooks(t *testing.T) {
	t.Helper()

	origResolveAuth := resolveAuthFn
	origResolveAuthStatePath := resolveAuthStatePathFn
	origSaveAuthState := saveAuthStateFn
	origNewAuthClient := newAuthClientFn

	t.Cleanup(func() {
		resolveAuthFn = origResolveAuth
		resolveAuthStatePathFn = origResolveAuthStatePath
		saveAuthStateFn = origSaveAuthState
		newAuthClientFn = origNewAuthClient
	})
}

func TestCommandWiring(t *testing.T) {
	root := newRootCmd()

	loginCmd, _, err := root.Find([]string{"login"})
	if err != nil {
		t.Fatalf("Find(login) error = %v", err)
	}
	if loginCmd == nil || loginCmd.Name() != "login" {
		t.Fatalf("login command not wired correctly")
	}

	logoutCmd, _, err := root.Find([]string{"logout"})
	if err != nil {
		t.Fatalf("Find(logout) error = %v", err)
	}
	if logoutCmd == nil || logoutCmd.Name() != "logout" {
		t.Fatalf("logout command not wired correctly")
	}

	statusCmd, _, err := root.Find([]string{"auth", "status"})
	if err != nil {
		t.Fatalf("Find(auth status) error = %v", err)
	}
	if statusCmd == nil || statusCmd.Name() != "status" {
		t.Fatalf("auth status command not wired correctly")
	}
}

func TestAuthStatusCommand(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv("CHATGPT_BULK_AUTH_FILE", authPath)
	t.Setenv("CHATGPT_SESSION_TOKEN", "env-session")
	t.Setenv("CHATGPT_CSRF_TOKEN", "env-csrf")

	_, err := chatgpt.SaveAuthState(chatgpt.AuthState{
		SessionToken: "stored-session",
		CSRFToken:    "stored-csrf",
		AccessToken:  "stored-access",
		UserEmail:    "stored@example.com",
		SavedAt:      time.Date(2026, time.March, 24, 12, 0, 0, 0, time.UTC),
		Source:       "browser",
	})
	if err != nil {
		t.Fatalf("SaveAuthState() error = %v", err)
	}

	root := newRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stdout)
	root.SetArgs([]string{"auth", "status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"Stored auth: present",
		"Stored email: stored@example.com",
		"Env session token: present",
		"Env csrf token: present",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestLogoutCommandRemovesStoredAuth(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv("CHATGPT_BULK_AUTH_FILE", authPath)

	_, err := chatgpt.SaveAuthState(chatgpt.AuthState{
		SessionToken: "stored-session",
		SavedAt:      time.Date(2026, time.March, 24, 12, 30, 0, 0, time.UTC),
		Source:       "browser",
	})
	if err != nil {
		t.Fatalf("SaveAuthState() error = %v", err)
	}

	root := newRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stdout)
	root.SetArgs([]string{"logout"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatalf("stored auth file still exists after logout: %v", err)
	}
	if !strings.Contains(stdout.String(), "Removed stored auth") {
		t.Fatalf("logout output missing confirmation:\n%s", stdout.String())
	}
}

func TestLoginCommandEndToEnd(t *testing.T) {
	withMainTestHooks(t)

	authPath := filepath.Join(t.TempDir(), "auth.json")
	resolvedState := chatgpt.AuthState{
		SessionToken: "existing-session",
		CSRFToken:    "existing-csrf",
		UserEmail:    "existing@example.com",
	}
	authenticatedState := chatgpt.AuthState{
		SessionToken: "new-session",
		CSRFToken:    "new-csrf",
		AccessToken:  "new-access",
		UserEmail:    "new@example.com",
		SavedAt:      time.Date(2026, time.March, 24, 14, 0, 0, 0, time.UTC),
		Source:       "browser",
	}

	var (
		gotConfig  chatgpt.Config
		savedState chatgpt.AuthState
	)
	client := &stubAuthClient{
		authenticate: func(ctx context.Context) (chatgpt.AuthState, error) {
			if err := gotConfig.SaveAuth(authenticatedState); err != nil {
				return chatgpt.AuthState{}, err
			}
			return authenticatedState, nil
		},
	}

	resolveAuthFn = func(config chatgpt.Config) (chatgpt.ResolvedAuth, error) {
		return chatgpt.ResolvedAuth{
			State:  resolvedState,
			Source: chatgpt.AuthSourceStored,
			Path:   authPath,
		}, nil
	}
	resolveAuthStatePathFn = func() (string, error) {
		return authPath, nil
	}
	saveAuthStateFn = func(state chatgpt.AuthState) (string, error) {
		savedState = state
		return authPath, nil
	}
	newAuthClientFn = func(config chatgpt.Config) (authClient, error) {
		gotConfig = config
		return client, nil
	}

	root := newRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stdout)
	root.SetArgs([]string{"login", "--debug"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if gotConfig.SessionToken != resolvedState.SessionToken {
		t.Fatalf("client config session token = %q, want %q", gotConfig.SessionToken, resolvedState.SessionToken)
	}
	if !gotConfig.AllowLogin {
		t.Fatalf("client config AllowLogin = false, want true")
	}
	if !gotConfig.Debug {
		t.Fatalf("client config Debug = false, want true")
	}
	if savedState != authenticatedState {
		t.Fatalf("saved auth state = %#v, want %#v", savedState, authenticatedState)
	}
	if client.closeCalls != 1 {
		t.Fatalf("client close calls = %d, want 1", client.closeCalls)
	}

	output := stdout.String()
	for _, want := range []string{
		"Logged in as new@example.com",
		"Saved auth to " + authPath,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("login output missing %q:\n%s", want, output)
		}
	}
}

func TestLoginCommandPropagatesSaveError(t *testing.T) {
	withMainTestHooks(t)

	expectedErr := errors.New("disk full")

	resolveAuthFn = func(config chatgpt.Config) (chatgpt.ResolvedAuth, error) {
		return chatgpt.ResolvedAuth{}, nil
	}
	resolveAuthStatePathFn = func() (string, error) {
		return filepath.Join(t.TempDir(), "auth.json"), nil
	}
	saveAuthStateFn = func(state chatgpt.AuthState) (string, error) {
		return "", expectedErr
	}
	newAuthClientFn = func(config chatgpt.Config) (authClient, error) {
		return &stubAuthClient{
			authenticate: func(ctx context.Context) (chatgpt.AuthState, error) {
				err := config.SaveAuth(chatgpt.AuthState{SessionToken: "session"})
				if err == nil {
					t.Fatalf("SaveAuth() error = nil, want %v", expectedErr)
				}
				return chatgpt.AuthState{}, err
			},
		}, nil
	}

	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"login"})

	err := root.Execute()
	if err == nil {
		t.Fatalf("Execute() error = nil, want %v", expectedErr)
	}
	if !strings.Contains(err.Error(), expectedErr.Error()) {
		t.Fatalf("Execute() error = %v, want substring %q", err, expectedErr.Error())
	}
}
