package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
)

type config struct {
	binaryPath        string
	chromePath        string
	authPath          string
	timeout           time.Duration
	headless          bool
	debug             bool
	keepArtifacts     bool
	verifySessionOnly bool
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "auth-e2e harness failed: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.binaryPath, "binary", "", "Path to an existing chatgpt-bulk binary; default builds a temp binary from the repo")
	flag.StringVar(&cfg.chromePath, "chrome-path", "", "Optional Chrome/Edge/Brave executable path forwarded to chatgpt-bulk")
	flag.StringVar(&cfg.authPath, "auth-file", "", "Optional auth file path; default uses a temp file")
	flag.DurationVar(&cfg.timeout, "timeout", 10*time.Minute, "Overall timeout for each interactive auth step")
	flag.BoolVar(&cfg.headless, "headless", false, "Forward --headless to chatgpt-bulk")
	flag.BoolVar(&cfg.debug, "debug", false, "Forward --debug to chatgpt-bulk")
	flag.BoolVar(&cfg.keepArtifacts, "keep-artifacts", false, "Keep the temp auth file and temp binary instead of cleaning them up")
	flag.BoolVar(&cfg.verifySessionOnly, "verify-session-only", false, "Also run a manual-assisted session-only auth check")
	flag.Parse()
	return cfg
}

func run(cfg config) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}

	authPath, cleanupAuth, err := prepareAuthPath(cfg.authPath)
	if err != nil {
		return err
	}
	if !cfg.keepArtifacts {
		defer cleanupAuth()
	}

	binaryPath, cleanupBinary, err := prepareBinary(repoRoot, cfg.binaryPath)
	if err != nil {
		return err
	}
	if !cfg.keepArtifacts {
		defer cleanupBinary()
	}

	fmt.Printf("Repository root: %s\n", repoRoot)
	fmt.Printf("Auth file under test: %s\n", authPath)
	fmt.Printf("chatgpt-bulk binary: %s\n", binaryPath)
	fmt.Println()

	if err := runPermanentFlow(binaryPath, authPath, cfg); err != nil {
		return err
	}

	if cfg.verifySessionOnly {
		fmt.Println()
		if err := runSessionOnlyFlow(authPath, cfg); err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Println("Manual-assisted auth E2E verification completed successfully.")
	return nil
}

func prepareAuthPath(explicit string) (string, func(), error) {
	if strings.TrimSpace(explicit) != "" {
		path := filepath.Clean(explicit)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("remove existing auth file: %w", err)
		}
		return path, func() {}, nil
	}

	dir, err := os.MkdirTemp("", "chatgpt-bulk-auth-e2e-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	path := filepath.Join(dir, "auth.json")
	cleanup := func() { _ = os.RemoveAll(dir) }
	return path, cleanup, nil
}

func prepareBinary(repoRoot, explicit string) (string, func(), error) {
	if strings.TrimSpace(explicit) != "" {
		return filepath.Clean(explicit), func() {}, nil
	}

	dir, err := os.MkdirTemp("", "chatgpt-bulk-bin-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp binary dir: %w", err)
	}

	name := "chatgpt-bulk"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binaryPath := filepath.Join(dir, name)

	fmt.Println("Building chatgpt-bulk for the harness...")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/chatgpt-bulk")
	buildCmd.Dir = repoRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("build chatgpt-bulk: %w", err)
	}

	cleanup := func() { _ = os.RemoveAll(dir) }
	return binaryPath, cleanup, nil
}

func runPermanentFlow(binaryPath, authPath string, cfg config) error {
	fmt.Println("=== Permanent auth-file flow ===")
	fmt.Println("The harness will launch `chatgpt-bulk login --permanent` with a temporary auth file.")
	fmt.Println("Complete ChatGPT sign-in and any browser challenges when the browser opens.")
	fmt.Printf("Waiting up to %s for login to finish.\n", cfg.timeout)
	fmt.Println()

	if err := runLoginCommand(binaryPath, authPath, cfg, "--permanent"); err != nil {
		return fmt.Errorf("permanent login flow: %w", err)
	}

	if err := waitForAuthFile(authPath, 5*time.Second); err != nil {
		return err
	}
	fmt.Printf("Auth file created: %s\n", authPath)

	statusOutput, err := runStatusCommand(binaryPath, authPath)
	if err != nil {
		return err
	}
	if !authStatusLooksPresent(statusOutput) {
		return fmt.Errorf("auth status did not report stored auth present:\n%s", statusOutput)
	}
	fmt.Println("`chatgpt-bulk auth status` confirmed stored auth is present.")

	if err := verifySavedAuthReuse(authPath, cfg.timeout); err != nil {
		return err
	}
	fmt.Println("Stored auth reuse verified through the same resolve/new/authenticate path used before the TUI starts.")

	return nil
}

func runSessionOnlyFlow(authPath string, cfg config) error {
	fmt.Println("=== Session-only flow ===")
	if err := os.Remove(authPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear auth file before session-only flow: %w", err)
	}

	fmt.Println("The harness will now run an in-memory-only browser auth using the internal client.")
	fmt.Println("Complete ChatGPT sign-in and any browser challenges when the browser opens.")
	fmt.Printf("Waiting up to %s for login to finish.\n", cfg.timeout)
	fmt.Println()

	restoreEnv, err := setAuthFileEnv(authPath)
	if err != nil {
		return err
	}
	defer restoreEnv()

	client, err := chatgpt.New(chatgpt.Config{
		Debug:      cfg.debug,
		Headless:   cfg.headless,
		ChromePath: cfg.chromePath,
		AllowLogin: true,
	})
	if err != nil {
		return fmt.Errorf("create session-only client: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	state, err := client.Authenticate(ctx)
	if err != nil {
		return fmt.Errorf("session-only authenticate: %w", err)
	}
	fmt.Printf("Session-only auth succeeded for %s\n", displayEmail(state.UserEmail))

	if _, err := os.Stat(authPath); err == nil {
		return fmt.Errorf("session-only flow unexpectedly wrote auth file %s", authPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check session-only auth file: %w", err)
	}
	fmt.Println("No auth file was written during session-only verification.")

	return nil
}

func runLoginCommand(binaryPath, authPath string, cfg config, modeFlag string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	args := []string{"login", modeFlag}
	if cfg.chromePath != "" {
		args = append(args, "--chrome-path", cfg.chromePath)
	}
	if cfg.headless {
		args = append(args, "--headless")
	}
	if cfg.debug {
		args = append(args, "--debug")
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Env = append(os.Environ(), "CHATGPT_BULK_AUTH_FILE="+authPath)
	cmd.Stdout = io.MultiWriter(os.Stdout)
	cmd.Stderr = io.MultiWriter(os.Stderr)
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("login command timed out after %s", cfg.timeout)
		}
		return err
	}

	return nil
}

func waitForAuthFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		info, err := os.Stat(path)
		if err == nil {
			if info.Size() == 0 {
				return fmt.Errorf("auth file %s was created but is empty", path)
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("check auth file: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("auth file %s was not created within %s", path, timeout)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func runStatusCommand(binaryPath, authPath string) (string, error) {
	cmd := exec.Command(binaryPath, "auth", "status")
	cmd.Env = append(os.Environ(), "CHATGPT_BULK_AUTH_FILE="+authPath)
	output, err := cmd.CombinedOutput()
	text := string(output)
	fmt.Println("`chatgpt-bulk auth status` output:")
	fmt.Println(text)
	if err != nil {
		return text, fmt.Errorf("auth status command failed: %w", err)
	}
	return text, nil
}

func authStatusLooksPresent(output string) bool {
	return strings.Contains(output, "Stored auth: present")
}

func verifySavedAuthReuse(authPath string, timeout time.Duration) error {
	restoreEnv, err := setAuthFileEnv(authPath)
	if err != nil {
		return err
	}
	defer restoreEnv()

	resolved, err := chatgpt.ResolveAuth()
	if err != nil {
		return fmt.Errorf("resolve saved auth: %w", err)
	}
	if resolved.Source != chatgpt.AuthSourceStored {
		return fmt.Errorf("resolved auth source = %q, want %q", resolved.Source, chatgpt.AuthSourceStored)
	}

	client, err := chatgpt.New(chatgpt.Config{
		SessionToken: resolved.State.SessionToken,
		CSRFToken:    resolved.State.CSRFToken,
		AccessToken:  resolved.State.AccessToken,
		UserEmail:    resolved.State.UserEmail,
		AuthSource:   resolved.Source,
	})
	if err != nil {
		return fmt.Errorf("create reuse-verification client: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	state, err := client.Authenticate(ctx)
	if err != nil {
		return fmt.Errorf("authenticate with saved auth: %w", err)
	}

	fmt.Printf("Saved auth reused successfully for %s\n", displayEmail(state.UserEmail))
	return nil
}

func setAuthFileEnv(path string) (func(), error) {
	prev, hadPrev := os.LookupEnv("CHATGPT_BULK_AUTH_FILE")
	if err := os.Setenv("CHATGPT_BULK_AUTH_FILE", path); err != nil {
		return nil, fmt.Errorf("set CHATGPT_BULK_AUTH_FILE: %w", err)
	}
	return func() {
		if hadPrev {
			_ = os.Setenv("CHATGPT_BULK_AUTH_FILE", prev)
			return
		}
		_ = os.Unsetenv("CHATGPT_BULK_AUTH_FILE")
	}, nil
}

func displayEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "unknown email"
	}
	return email
}
