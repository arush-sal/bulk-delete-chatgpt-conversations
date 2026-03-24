package main

import (
	"context"
	"errors"
	"fmt"
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

type authClient interface {
	Authenticate(ctx context.Context) (chatgpt.AuthState, error)
	Close()
}

var (
	resolveAuthFn          = chatgpt.ResolveAuth
	resolveAuthStatePathFn = chatgpt.ResolveAuthStatePath
	saveAuthStateFn        = chatgpt.SaveAuthState
	newAuthClientFn        = func(config chatgpt.Config) (authClient, error) { return chatgpt.New(config) }
)

type commandOptions struct {
	chromePath string
	headless   bool
	debug      bool
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

			program := tea.NewProgram(tui.New(client, version.Short()))
			if _, err := program.Run(); err != nil {
				return fmt.Errorf("tui error: %w", err)
			}
			return nil
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
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with ChatGPT in a browser and save local auth state",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()

			resolved, err := resolveAuthFn(chatgpt.Config{})
			if err != nil {
				return err
			}

			client, err := newAuthClientFn(chatgpt.Config{
				SessionToken: resolved.State.SessionToken,
				CSRFToken:    resolved.State.CSRFToken,
				UserEmail:    resolved.State.UserEmail,
				AuthSource:   resolved.Source,
				Debug:        resolvedDebug(cmd, opts),
				Headless:     opts.headless,
				ChromePath:   opts.chromePath,
				AllowLogin:   true,
				SaveAuth: func(state chatgpt.AuthState) error {
					_, err := saveAuthStateFn(state)
					return err
				},
			})
			if err != nil {
				return err
			}
			defer client.Close()

			ctx, cancel := context.WithTimeout(cmd.Context(), 3*time.Minute)
			defer cancel()

			state, err := client.Authenticate(ctx)
			if err != nil {
				return err
			}

			path, pathErr := resolveAuthStatePathFn()
			if pathErr != nil {
				return pathErr
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s\nSaved auth to %s\n", displayValue(state.UserEmail, "unknown email"), path)
			return nil
		},
	}
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
		Short: "Show stored and environment-based ChatGPT auth availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = godotenv.Load()

			path, err := chatgpt.ResolveAuthStatePath()
			if err != nil {
				return fmt.Errorf("auth file path resolution failed: %w", err)
			}

			state, _, loadErr := chatgpt.LoadAuthState()
			hasStored := loadErr == nil
			if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
				return fmt.Errorf("auth file read failed: %w", loadErr)
			}

			envSession := strings.TrimSpace(os.Getenv("CHATGPT_SESSION_TOKEN"))
			envCSRF := strings.TrimSpace(os.Getenv("CHATGPT_CSRF_TOKEN"))

			if hasStored {
				fmt.Fprintf(cmd.OutOrStdout(), "Stored auth: present\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Auth file: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "Stored email: %s\n", displayValue(state.UserEmail, "unknown"))
				fmt.Fprintf(cmd.OutOrStdout(), "Last refreshed: %s\n", state.SavedAt.Format(time.RFC3339))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Stored auth: absent\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Auth file: %s\n", path)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Env session token: %s\n", presenceLabel(envSession != ""))
			fmt.Fprintf(cmd.OutOrStdout(), "Env csrf token: %s\n", presenceLabel(envCSRF != ""))
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
	resolved, err := resolveAuthFn(chatgpt.Config{})
	if err != nil {
		return nil, fmt.Errorf("auth file read failed: %w", err)
	}
	if resolved.Source == chatgpt.AuthSourceNone {
		return nil, errors.New("no stored ChatGPT auth found and CHATGPT_SESSION_TOKEN is not set; run `chatgpt-bulk login`")
	}

	client, err := chatgpt.New(chatgpt.Config{
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

func presenceLabel(ok bool) string {
	if ok {
		return "present"
	}
	return "absent"
}

func displayValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
