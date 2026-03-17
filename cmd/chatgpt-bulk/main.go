package main

import (
	"fmt"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/tui"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	client, err := chatgpt.NewFromEnv()
	if err != nil {
		fatalWithSetup(err)
	}
	defer client.Close()

	program := tea.NewProgram(tui.New(client, versionString()))
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}

func fatalWithSetup(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
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
