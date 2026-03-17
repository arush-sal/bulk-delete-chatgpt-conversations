package main

import (
	"fmt"
	"os"

	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	client, err := chatgpt.NewFromEnv()
	if err != nil {
		fatalWithSetup(err)
	}
	defer client.Close()

	program := tea.NewProgram(tui.New(client))
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}

func fatalWithSetup(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	if os.Getenv("CHATGPT_SESSION_TOKEN") == "" {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Set CHATGPT_SESSION_TOKEN in your environment or a .env file.")
		fmt.Fprintln(os.Stderr, "Find it in your browser at chatgpt.com under:")
		fmt.Fprintln(os.Stderr, "Developer Tools -> Application -> Storage -> Cookies -> __Secure-next-auth.session-token")
	}
	os.Exit(1)
}
