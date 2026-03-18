package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/tui"
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
	Version: versionString(),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := chatgpt.New(chatgpt.Config{
			Debug:      debugFlag,
			Headless:   headless,
			ChromePath: chromePath,
		})
		if err != nil {
			return err
		}
		defer client.Close()

		program := tea.NewProgram(tui.New(client, versionString()))
		if _, err := program.Run(); err != nil {
			return fmt.Errorf("tui error: %w", err)
		}
		return nil
	},
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

func versionString() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
