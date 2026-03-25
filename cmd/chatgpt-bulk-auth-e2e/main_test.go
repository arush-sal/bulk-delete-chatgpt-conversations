package main

import (
	"reflect"
	"testing"
	"time"
)

func TestParseFlagsFromArgsDefaultsToFullSuite(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlagsFromArgs(nil)
	if err != nil {
		t.Fatalf("parseFlagsFromArgs(nil) error = %v", err)
	}

	wantStages := []string{"missing-auth", "session-only", "permanent"}
	if got := cfg.selectedStages(); !reflect.DeepEqual(got, wantStages) {
		t.Fatalf("selectedStages() = %v, want %v", got, wantStages)
	}
}

func TestParseFlagsFromArgsRunsOnlyExplicitStages(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlagsFromArgs([]string{"--session-only", "--permanent"})
	if err != nil {
		t.Fatalf("parseFlagsFromArgs(explicit stages) error = %v", err)
	}

	wantStages := []string{"session-only", "permanent"}
	if got := cfg.selectedStages(); !reflect.DeepEqual(got, wantStages) {
		t.Fatalf("selectedStages() = %v, want %v", got, wantStages)
	}
}

func TestParseFlagsFromArgsParsesSharedOptions(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlagsFromArgs([]string{
		"--binary", "/tmp/chatgpt-bulk",
		"--auth-file", "/tmp/auth.json",
		"--chrome-path", "/tmp/chrome",
		"--timeout", "2m30s",
		"--headless",
		"--debug",
		"--keep-artifacts",
		"--missing-auth",
	})
	if err != nil {
		t.Fatalf("parseFlagsFromArgs(shared options) error = %v", err)
	}

	if cfg.binaryPath != "/tmp/chatgpt-bulk" {
		t.Fatalf("binaryPath = %q, want %q", cfg.binaryPath, "/tmp/chatgpt-bulk")
	}
	if cfg.authPath != "/tmp/auth.json" {
		t.Fatalf("authPath = %q, want %q", cfg.authPath, "/tmp/auth.json")
	}
	if cfg.chromePath != "/tmp/chrome" {
		t.Fatalf("chromePath = %q, want %q", cfg.chromePath, "/tmp/chrome")
	}
	if cfg.timeout != 150*time.Second {
		t.Fatalf("timeout = %s, want %s", cfg.timeout, 150*time.Second)
	}
	if !cfg.headless {
		t.Fatal("headless = false, want true")
	}
	if !cfg.debug {
		t.Fatal("debug = false, want true")
	}
	if !cfg.keepArtifacts {
		t.Fatal("keepArtifacts = false, want true")
	}

	wantStages := []string{"missing-auth"}
	if got := cfg.selectedStages(); !reflect.DeepEqual(got, wantStages) {
		t.Fatalf("selectedStages() = %v, want %v", got, wantStages)
	}
}

func TestAuthStatusLooksPresent(t *testing.T) {
	t.Parallel()

	if !authStatusLooksPresent("Stored auth: present\nAuth file: /tmp/auth.json\n") {
		t.Fatal("authStatusLooksPresent() = false, want true")
	}
	if authStatusLooksPresent("Stored auth: absent\n") {
		t.Fatal("authStatusLooksPresent() = true for absent auth, want false")
	}
}

func TestDisplayEmail(t *testing.T) {
	t.Parallel()

	if got := displayEmail(" user@example.com "); got != "user@example.com" {
		t.Fatalf("displayEmail(trimmed) = %q, want %q", got, "user@example.com")
	}
	if got := displayEmail(" \t "); got != "unknown email" {
		t.Fatalf("displayEmail(blank) = %q, want %q", got, "unknown email")
	}
}
