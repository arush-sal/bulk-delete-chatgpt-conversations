package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
)

func TestNewAppClientPromptsForPermanentAuthWhenStoredAuthMissing(t *testing.T) {
	restore := overrideCommandGlobals(t)
	defer restore()

	resolveAuthFn = func() (chatgpt.ResolvedAuth, error) {
		return chatgpt.ResolvedAuth{Source: chatgpt.AuthSourceNone, Path: "/tmp/auth.json"}, nil
	}
	isTerminalFn = func(_ io.Reader, _ io.Writer) bool { return true }

	var captured chatgpt.Config
	newChatGPTClientFn = func(config chatgpt.Config) (*chatgpt.Client, error) {
		captured = config
		return &chatgpt.Client{}, nil
	}

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetIn(strings.NewReader("yes\n"))

	client, err := newAppClient(cmd, &commandOptions{})
	if err != nil {
		t.Fatalf("newAppClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("newAppClient() client = nil")
	}
	if !captured.AllowLogin {
		t.Fatal("AllowLogin = false, want true")
	}
	if captured.SaveAuth == nil {
		t.Fatal("SaveAuth = nil, want save callback for permanent auth")
	}

	output := stdout.String()
	if !strings.Contains(output, "Stored ChatGPT auth file not found at /tmp/auth.json") {
		t.Fatalf("stdout = %q, want missing auth file notice", output)
	}
	if !strings.Contains(output, "Use `chatgpt-bulk login` for short-lived sessions.") {
		t.Fatalf("stdout = %q, want short-lived login guidance", output)
	}
}

func TestNewAppClientKeepsAuthInMemoryWhenUserDeclinesPermanentFile(t *testing.T) {
	restore := overrideCommandGlobals(t)
	defer restore()

	resolveAuthFn = func() (chatgpt.ResolvedAuth, error) {
		return chatgpt.ResolvedAuth{Source: chatgpt.AuthSourceNone, Path: "/tmp/auth.json"}, nil
	}
	isTerminalFn = func(_ io.Reader, _ io.Writer) bool { return true }

	var captured chatgpt.Config
	newChatGPTClientFn = func(config chatgpt.Config) (*chatgpt.Client, error) {
		captured = config
		return &chatgpt.Client{}, nil
	}

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetIn(strings.NewReader("n\n"))

	_, err := newAppClient(cmd, &commandOptions{})
	if err != nil {
		t.Fatalf("newAppClient() error = %v", err)
	}
	if captured.SaveAuth != nil {
		t.Fatal("SaveAuth != nil, want in-memory-only auth")
	}
	if !strings.Contains(stdout.String(), "Proceeding with in-memory auth for this session only.") {
		t.Fatalf("stdout = %q, want in-memory auth message", stdout.String())
	}
}

func TestNewAppClientReturnsHelpfulErrorWhenPromptIsNotPossible(t *testing.T) {
	restore := overrideCommandGlobals(t)
	defer restore()

	resolveAuthFn = func() (chatgpt.ResolvedAuth, error) {
		return chatgpt.ResolvedAuth{Source: chatgpt.AuthSourceNone, Path: "/tmp/auth.json"}, nil
	}
	isTerminalFn = func(_ io.Reader, _ io.Writer) bool { return false }

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	_, err := newAppClient(cmd, &commandOptions{})
	if err == nil {
		t.Fatal("newAppClient() error = nil, want missing auth error")
	}
	if !strings.Contains(err.Error(), "stored ChatGPT auth file is missing at /tmp/auth.json; run `chatgpt-bulk login` to authenticate") {
		t.Fatalf("error = %v, want missing auth guidance", err)
	}
}

func TestLoginCommandSavesPermanentAuthWhenUserChoosesYes(t *testing.T) {
	restore := overrideCommandGlobals(t)
	defer restore()

	resolveAuthFn = func() (chatgpt.ResolvedAuth, error) {
		return chatgpt.ResolvedAuth{Source: chatgpt.AuthSourceNone, Path: "/tmp/auth.json"}, nil
	}
	resolveAuthStatePathFn = func() (string, error) { return "/tmp/auth.json", nil }
	isTerminalFn = func(_ io.Reader, _ io.Writer) bool { return true }

	var captured chatgpt.Config
	newChatGPTClientFn = func(config chatgpt.Config) (*chatgpt.Client, error) {
		captured = config
		return &chatgpt.Client{}, nil
	}
	authenticateClientFn = func(_ *chatgpt.Client, _ context.Context) (chatgpt.AuthState, error) {
		return chatgpt.AuthState{UserEmail: "user@example.com"}, nil
	}

	var tuiRan bool
	runTUIFn = func(_ *chatgpt.Client) error {
		tuiRan = true
		return nil
	}

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{"login"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if captured.SaveAuth == nil {
		t.Fatal("SaveAuth = nil, want persistent auth")
	}
	if tuiRan {
		t.Fatal("runTUIFn() called for permanent login, want false")
	}
	if !strings.Contains(stdout.String(), "Saved auth to /tmp/auth.json") {
		t.Fatalf("stdout = %q, want saved auth message", stdout.String())
	}
}

func TestLoginCommandStartsSessionOnlyFlowWhenUserChoosesNo(t *testing.T) {
	restore := overrideCommandGlobals(t)
	defer restore()

	resolveAuthFn = func() (chatgpt.ResolvedAuth, error) {
		return chatgpt.ResolvedAuth{Source: chatgpt.AuthSourceNone, Path: "/tmp/auth.json"}, nil
	}
	resolveAuthStatePathFn = func() (string, error) { return "/tmp/auth.json", nil }
	isTerminalFn = func(_ io.Reader, _ io.Writer) bool { return true }

	var captured chatgpt.Config
	newChatGPTClientFn = func(config chatgpt.Config) (*chatgpt.Client, error) {
		captured = config
		return &chatgpt.Client{}, nil
	}
	authenticateClientFn = func(_ *chatgpt.Client, _ context.Context) (chatgpt.AuthState, error) {
		return chatgpt.AuthState{UserEmail: "user@example.com"}, nil
	}

	var tuiRan bool
	runTUIFn = func(_ *chatgpt.Client) error {
		tuiRan = true
		return nil
	}

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetIn(strings.NewReader("no\n"))
	cmd.SetArgs([]string{"login"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if captured.SaveAuth == nil {
		t.Fatal("SaveAuth = nil, want configured auth callback")
	}
	if !tuiRan {
		t.Fatal("runTUIFn() not called for session-only login")
	}
	if !strings.Contains(stdout.String(), "Auth will be kept in-memory for this session only.") {
		t.Fatalf("stdout = %q, want in-memory auth message", stdout.String())
	}
}

func overrideCommandGlobals(t *testing.T) func() {
	t.Helper()

	origResolveAuthFn := resolveAuthFn
	origResolveAuthStatePathFn := resolveAuthStatePathFn
	origSaveAuthStateFn := saveAuthStateFn
	origNewChatGPTClientFn := newChatGPTClientFn
	origAuthenticateClientFn := authenticateClientFn
	origRunTUIFn := runTUIFn
	origIsTerminalFn := isTerminalFn

	saveAuthStateFn = func(chatgpt.AuthState) (string, error) { return "/tmp/auth.json", nil }

	return func() {
		resolveAuthFn = origResolveAuthFn
		resolveAuthStatePathFn = origResolveAuthStatePathFn
		saveAuthStateFn = origSaveAuthStateFn
		newChatGPTClientFn = origNewChatGPTClientFn
		authenticateClientFn = origAuthenticateClientFn
		runTUIFn = origRunTUIFn
		isTerminalFn = origIsTerminalFn
	}
}
