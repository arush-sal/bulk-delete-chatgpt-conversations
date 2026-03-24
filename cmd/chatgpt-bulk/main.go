package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/tui"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/version"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	resolveAuthFn          = chatgpt.ResolveAuth
	resolveAuthStatePathFn = chatgpt.ResolveAuthStatePath
	saveAuthStateFn        = chatgpt.SaveAuthState
	newChatGPTClientFn     = chatgpt.New
	authenticateClientFn   = func(client *chatgpt.Client, ctx context.Context) (chatgpt.AuthState, error) {
		return client.Authenticate(ctx)
	}
	runTUIFn = func(client *chatgpt.Client) error {
		program := tea.NewProgram(tui.New(client, version.Short()))
		if _, err := program.Run(); err != nil {
			return fmt.Errorf("tui error: %w", err)
		}
		return nil
	}
	isTerminalFn = func(r io.Reader, w io.Writer) bool {
		inFile, inOK := r.(*os.File)
		outFile, outOK := w.(*os.File)
		if !inOK || !outOK {
			return false
		}
		inInfo, err := inFile.Stat()
		if err != nil {
			return false
		}
		outInfo, err := outFile.Stat()
		if err != nil {
			return false
		}
		return inInfo.Mode()&os.ModeCharDevice != 0 && outInfo.Mode()&os.ModeCharDevice != 0
	}
)

type commandOptions struct {
	chromePath string
	headless   bool
	debug      bool
}

type loginOptions struct {
	permanent   bool
	sessionOnly bool
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	opts := &commandOptions{}

	rootCmd := &cobra.Command{
		Use:     "chatgpt-bulk",
		Short:   "Bulk delete ChatGPT conversations",
		Long:    "A TUI application for bulk deleting ChatGPT conversations using browser automation.",
		Version: version.Full(),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()

			client, err := newAppClient(cmd, opts)
			if err != nil {
				return err
			}
			defer client.Close()

			return runTUIFn(client)
		},
	}

	rootCmd.PersistentFlags().StringVar(&opts.chromePath, "chrome-path", "", "Chrome/Edge executable path")
	rootCmd.PersistentFlags().BoolVar(&opts.headless, "headless", false, "Launch Chrome headless")
	rootCmd.PersistentFlags().BoolVar(&opts.debug, "debug", false, "Enable verbose debug logs in the TUI")

	rootCmd.AddCommand(newLoginCmd(opts))
	rootCmd.AddCommand(newLogoutCmd())
	rootCmd.AddCommand(newAuthCmd())

	return rootCmd
}

func newLoginCmd(opts *commandOptions) *cobra.Command {
	loginOpts := &loginOptions{}

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with ChatGPT for a saved or session-only login",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()

			mode, err := resolveLoginMode(cmd, loginOpts)
			if err != nil {
				return err
			}

			if mode == loginModePrompt {
				path, err := resolveAuthStatePathFn()
				if err != nil {
					return err
				}
				saveAuth, err := promptToSaveAuth(cmd, path)
				if err != nil {
					return err
				}
				if saveAuth {
					mode = loginModePermanent
				} else {
					mode = loginModeSessionOnly
				}
			}

			resolved, err := resolveAuthFn()
			if err != nil {
				return err
			}

			saveAuth := func(state chatgpt.AuthState) error { return nil }
			if mode == loginModePermanent {
				saveAuth = func(state chatgpt.AuthState) error {
					_, err := saveAuthStateFn(state)
					return err
				}
			}

			client, err := newChatGPTClientFn(chatgpt.Config{
				SessionToken: resolved.State.SessionToken,
				CSRFToken:    resolved.State.CSRFToken,
				AccessToken:  resolved.State.AccessToken,
				UserEmail:    resolved.State.UserEmail,
				AuthSource:   resolved.Source,
				Debug:        resolvedDebug(cmd, opts),
				Headless:     opts.headless,
				ChromePath:   opts.chromePath,
				AllowLogin:   true,
				SaveAuth:     saveAuth,
			})
			if err != nil {
				return err
			}
			defer client.Close()

			ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Minute)
			defer cancel()

			state, err := authenticateClientFn(client, ctx)
			if err != nil {
				return err
			}

			if mode == loginModeSessionOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s\nAuth will be kept in-memory for this session only.\n", displayValue(state.UserEmail, "unknown email"))
				return runTUIFn(client)
			}

			path, err := resolveAuthStatePathFn()
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s\nSaved auth to %s\n", displayValue(state.UserEmail, "unknown email"), path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&loginOpts.permanent, "permanent", false, "Save auth to the default auth file")
	cmd.Flags().BoolVar(&loginOpts.sessionOnly, "session-only", false, "Keep auth in-memory only and open the TUI for this session")
	return cmd
}

func newAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Inspect stored ChatGPT authentication state",
	}
	authCmd.AddCommand(newAuthStatusCmd())
	return authCmd
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show stored ChatGPT auth availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := chatgpt.ResolveAuthStatePath()
			if err != nil {
				return fmt.Errorf("auth file path resolution failed: %w", err)
			}

			state, _, loadErr := chatgpt.LoadAuthState()
			hasStored := loadErr == nil
			if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
				return fmt.Errorf("auth file read failed: %w", loadErr)
			}

			if hasStored {
				fmt.Fprintf(cmd.OutOrStdout(), "Stored auth: present\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Auth file: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "Stored email: %s\n", displayValue(state.UserEmail, "unknown"))
				fmt.Fprintf(cmd.OutOrStdout(), "Last refreshed: %s\n", state.SavedAt.Format(time.RFC3339))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Stored auth: absent\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Auth file: %s\n", path)
			}
			return nil
		},
	}
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored ChatGPT auth state",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, removed, err := chatgpt.RemoveAuthState()
			if err != nil {
				return fmt.Errorf("logout failed: %w", err)
			}
			if removed {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed stored auth from %s\n", path)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "No stored auth found at %s\n", path)
			return nil
		},
	}
}

func newAppClient(cmd *cobra.Command, opts *commandOptions) (*chatgpt.Client, error) {
	resolved, err := resolveAuthFn()
	if err != nil {
		return nil, fmt.Errorf("auth file read failed: %w", err)
	}
	if resolved.Source == chatgpt.AuthSourceNone {
		return newInteractiveLoginClient(cmd, opts, resolved.Path)
	}

	client, err := newChatGPTClientFn(chatgpt.Config{
		SessionToken: resolved.State.SessionToken,
		CSRFToken:    resolved.State.CSRFToken,
		AccessToken:  resolved.State.AccessToken,
		UserEmail:    resolved.State.UserEmail,
		AuthSource:   resolved.Source,
		Debug:        resolvedDebug(cmd, opts),
		Headless:     opts.headless,
		ChromePath:   opts.chromePath,
		SaveAuth: func(state chatgpt.AuthState) error {
			_, err := chatgpt.SaveAuthState(state)
			return err
		},
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

type loginMode int

const (
	loginModePrompt loginMode = iota
	loginModePermanent
	loginModeSessionOnly
)

func resolveLoginMode(cmd *cobra.Command, opts *loginOptions) (loginMode, error) {
	if opts.permanent && opts.sessionOnly {
		return loginModePrompt, errors.New("choose either --permanent or --session-only, not both")
	}
	if opts.permanent {
		return loginModePermanent, nil
	}
	if opts.sessionOnly {
		return loginModeSessionOnly, nil
	}
	if !isTerminalFn(cmd.InOrStdin(), cmd.OutOrStdout()) {
		return loginModePrompt, errors.New("login mode is ambiguous in a non-interactive terminal; pass --permanent or --session-only")
	}
	return loginModePrompt, nil
}

func newInteractiveLoginClient(cmd *cobra.Command, opts *commandOptions, authPath string) (*chatgpt.Client, error) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Stored ChatGPT auth file not found at %s\n", authPath)
	fmt.Fprintln(out, "Use `chatgpt-bulk login` for short-lived sessions.")

	if !isTerminalFn(cmd.InOrStdin(), out) {
		return nil, fmt.Errorf("stored ChatGPT auth file is missing at %s; run `chatgpt-bulk login` to authenticate", authPath)
	}

	saveAuth, err := promptToSaveAuth(cmd, authPath)
	if err != nil {
		return nil, err
	}

	config := chatgpt.Config{
		Debug:      resolvedDebug(cmd, opts),
		Headless:   opts.headless,
		ChromePath: opts.chromePath,
		AllowLogin: true,
	}
	if saveAuth {
		config.SaveAuth = func(state chatgpt.AuthState) error {
			_, err := saveAuthStateFn(state)
			return err
		}
	}

	return newChatGPTClientFn(config)
}

func promptToSaveAuth(cmd *cobra.Command, authPath string) (bool, error) {
	reader := bufio.NewReader(cmd.InOrStdin())
	for {
		fmt.Fprintf(cmd.OutOrStdout(), "Create a permanent auth file at %s? [y/N]: ", authPath)

		answer, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, fmt.Errorf("read login choice: %w", err)
		}

		answer = strings.TrimSpace(strings.ToLower(answer))
		switch answer {
		case "y", "yes":
			return true, nil
		case "", "n", "no":
			fmt.Fprintln(cmd.OutOrStdout(), "Proceeding with in-memory auth for this session only.")
			return false, nil
		default:
			fmt.Fprintln(cmd.OutOrStdout(), "Please answer yes or no.")
			if errors.Is(err, io.EOF) {
				return false, errors.New("login choice ended before receiving yes or no")
			}
		}
	}
}

func resolvedDebug(cmd *cobra.Command, opts *commandOptions) bool {
	flag := cmd.Flags().Lookup("debug")
	if flag != nil && flag.Changed {
		return opts.debug
	}
	return opts.debug || parseBoolEnv("DEBUG")
}

func parseBoolEnv(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	ok, err := strconv.ParseBool(v)
	return err == nil && ok
}

func displayValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
