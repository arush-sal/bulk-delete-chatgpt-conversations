package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/tui"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/version"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	chromePath string
	headless   bool
	debugFlag  bool
)

var rootCmd = &cobra.Command{
	Use:     "chatgpt-bulk",
	Short:   "Bulk delete ChatGPT conversations",
	Long:    "A TUI application for bulk deleting ChatGPT conversations using browser automation.",
	Version: version.Full(),
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = godotenv.Load()
		if !debugFlag {
			debugFlag = parseBoolEnv("DEBUG")
		}
		client, err := chatgpt.New(chatgpt.Config{
			Debug:      debugFlag,
			Headless:   headless,
			ChromePath: chromePath,
		})
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

func parseBoolEnv(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	ok, err := strconv.ParseBool(v)
	return err == nil && ok
}

func init() {
	rootCmd.Flags().StringVar(&chromePath, "chrome-path", "", "Chrome/Edge executable path")
	rootCmd.Flags().BoolVar(&headless, "headless", false, "Launch Chrome headless")
	rootCmd.Flags().BoolVar(&debugFlag, "debug", false, "Enable verbose debug logs in the TUI")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
