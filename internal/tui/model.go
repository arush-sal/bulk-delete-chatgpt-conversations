package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type phase int

const (
	phaseLoading phase = iota
	phaseSelect
	phaseAction
	phaseConfirm
	phaseRunning
	phaseDone
	phaseError
)

type actionChoice int

const (
	actionArchive actionChoice = iota
	actionDelete
	actionCancel
)

var actionLabels = []string{"Archive", "Delete", "Cancel"}

type loadResultMsg struct {
	conversations []chatgpt.Conversation
	err           error
}

type statusMsg string

type actionResult struct {
	id    string
	err   error
	label string
}

type actionFinishedMsg struct {
	results []actionResult
}

type Model struct {
	client        *chatgpt.Client
	width         int
	height        int
	phase         phase
	loadingText   string
	conversations []chatgpt.Conversation
	cursor        int
	selected      map[string]struct{}
	actionCursor  int
	runningIndex  int
	results       []actionResult
	err           error
}

func New(client *chatgpt.Client) Model {
	return Model{
		client:      client,
		phase:       phaseLoading,
		loadingText: "Launching Chrome and loading your ChatGPT session...\n\nIf Chrome opens a login page, finish signing in there and keep this terminal open.",
		selected:    make(map[string]struct{}),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadConversationsCmd(m.client), pollStatusCmd(m.client))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case loadResultMsg:
		if msg.err != nil {
			m.phase = phaseError
			m.err = msg.err
			return m, nil
		}
		m.conversations = msg.conversations
		m.phase = phaseSelect
		if len(m.conversations) == 0 {
			m.phase = phaseDone
		}
	case statusMsg:
		if m.phase == phaseLoading {
			text := strings.TrimSpace(string(msg))
			if text != "" {
				m.loadingText = text
			}
			return m, pollStatusCmd(m.client)
		}
	case actionFinishedMsg:
		m.results = msg.results
		m.phase = phaseDone
		return m, nil
	case tea.KeyMsg:
		switch m.phase {
		case phaseLoading, phaseRunning:
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return m, tea.Quit
			}
		case phaseSelect:
			return m.updateSelection(msg)
		case phaseAction:
			return m.updateActionPicker(msg)
		case phaseConfirm:
			return m.updateConfirmation(msg)
		case phaseDone, phaseError:
			if msg.String() == "enter" || msg.String() == "q" || msg.String() == "esc" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.phase {
	case phaseLoading:
		return m.frame(m.loadingText)
	case phaseSelect:
		return m.frame(m.renderSelection())
	case phaseAction:
		return m.frame(m.renderActionPicker())
	case phaseConfirm:
		return m.frame(m.renderConfirmation())
	case phaseRunning:
		return m.frame(m.renderRunning())
	case phaseDone:
		return m.frame(m.renderDone())
	case phaseError:
		return m.frame(renderError(m.err))
	default:
		return ""
	}
}

func (m Model) updateSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.conversations)-1 {
			m.cursor++
		}
	case " ":
		m.toggleCurrent()
	case "a":
		if len(m.selected) == len(m.conversations) {
			m.selected = make(map[string]struct{})
		} else {
			for _, conv := range m.conversations {
				m.selected[conv.ID] = struct{}{}
			}
		}
	case "enter":
		if len(m.selected) > 0 {
			m.phase = phaseAction
		}
	}
	return m, nil
}

func (m Model) updateActionPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case "down", "j":
		if m.actionCursor < len(actionLabels)-1 {
			m.actionCursor++
		}
	case "esc":
		m.phase = phaseSelect
	case "enter":
		if m.actionCursor == int(actionCancel) {
			m.phase = phaseSelect
			return m, nil
		}
		m.phase = phaseConfirm
	}
	return m, nil
}

func (m Model) updateConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "n", "esc":
		m.phase = phaseAction
	case "y", "enter":
		m.phase = phaseRunning
		return m, runBulkActionCmd(m.client, m.selectedConversations(), m.selectedAction())
	}
	return m, nil
}

func (m *Model) toggleCurrent() {
	if len(m.conversations) == 0 {
		return
	}
	id := m.conversations[m.cursor].ID
	if _, ok := m.selected[id]; ok {
		delete(m.selected, id)
		return
	}
	m.selected[id] = struct{}{}
}

func (m Model) selectedAction() actionChoice {
	return actionChoice(m.actionCursor)
}

func (m Model) selectedConversations() []chatgpt.Conversation {
	selected := make([]chatgpt.Conversation, 0, len(m.selected))
	for _, conv := range m.conversations {
		if _, ok := m.selected[conv.ID]; ok {
			selected = append(selected, conv)
		}
	}
	return selected
}

func (m Model) renderSelection() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("ChatGPT Conversations"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf("%d conversations loaded, %d selected", len(m.conversations), len(m.selected))))
	b.WriteString("\n\n")

	if len(m.conversations) == 0 {
		b.WriteString("No conversations found for this account.\n")
		b.WriteString(helpStyle.Render("Press q to quit."))
		return b.String()
	}

	visible := m.visibleRange()
	for i := visible.start; i < visible.end; i++ {
		conv := m.conversations[i]
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		mark := " "
		if _, ok := m.selected[conv.ID]; ok {
			mark = "x"
		}

		line := fmt.Sprintf("%s [%s] %s", cursor, mark, titleFor(conv))
		if i == m.cursor {
			b.WriteString(selectedLineStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("up/down: move  space: toggle  a: all/none  enter: continue  q: quit"))
	return b.String()
}

func (m Model) renderActionPicker() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Choose Action"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf("%d conversations selected", len(m.selected))))
	b.WriteString("\n\n")

	for i, label := range actionLabels {
		prefix := "  "
		if i == m.actionCursor {
			prefix = "> "
			b.WriteString(selectedLineStyle.Render(prefix + label))
		} else {
			b.WriteString(prefix + label)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("up/down: move  enter: choose  esc: back"))
	return b.String()
}

func (m Model) renderConfirmation() string {
	actionLabel := actionLabels[m.actionCursor]
	targets := m.selectedConversations()

	var b strings.Builder
	b.WriteString(titleStyle.Render("Confirm"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s %d conversations?\n\n", actionLabel, len(targets)))
	for i, conv := range targets {
		if i >= 8 {
			b.WriteString(subtleStyle.Render(fmt.Sprintf("...and %d more", len(targets)-i)))
			b.WriteString("\n")
			break
		}
		b.WriteString("• " + titleFor(conv) + "\n")
	}
	b.WriteString("\n")
	if m.selectedAction() == actionDelete {
		b.WriteString(warningStyle.Render("Delete marks conversations as not visible and may be hard to recover."))
	} else {
		b.WriteString(subtleStyle.Render("Archive hides selected conversations without deleting them."))
	}
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("y/enter: confirm  n/esc: back"))
	return b.String()
}

func (m Model) renderRunning() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Processing"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Applying %s to %d conversations...\n", strings.ToLower(actionLabels[m.actionCursor]), len(m.selected)))
	b.WriteString(subtleStyle.Render("Please wait."))
	return b.String()
}

func (m Model) renderDone() string {
	if len(m.results) == 0 && len(m.conversations) == 0 {
		return titleStyle.Render("No conversations found.") + "\n\n" + helpStyle.Render("Press q to quit.")
	}

	successes := 0
	for _, result := range m.results {
		if result.err == nil {
			successes++
		}
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Completed"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf("%d succeeded, %d failed", successes, len(m.results)-successes)))
	b.WriteString("\n\n")
	for _, result := range m.results {
		status := successStyle.Render("OK")
		if result.err != nil {
			status = errorStyle.Render("FAIL")
		}
		b.WriteString(fmt.Sprintf("[%s] %s (%s)", status, result.label, shortID(result.id)))
		if result.err != nil {
			b.WriteString("\n    " + subtleStyle.Render(result.err.Error()))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Press enter or q to exit."))
	return b.String()
}

func renderError(err error) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Error"))
	b.WriteString("\n\n")
	if err != nil {
		b.WriteString(errorStyle.Render(err.Error()))
		b.WriteString("\n\n")
	}
	b.WriteString(subtleStyle.Render("Check CHATGPT_SESSION_TOKEN, refresh it from your browser, and try again."))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Press enter or q to exit."))
	return b.String()
}

func (m Model) frame(content string) string {
	return docStyle.Width(max(80, m.width)).Render(content)
}

func (m Model) visibleRange() struct{ start, end int } {
	maxItems := max(8, m.height-8)
	if len(m.conversations) <= maxItems {
		return struct{ start, end int }{0, len(m.conversations)}
	}

	start := m.cursor - maxItems/2
	if start < 0 {
		start = 0
	}
	end := start + maxItems
	if end > len(m.conversations) {
		end = len(m.conversations)
		start = end - maxItems
	}
	return struct{ start, end int }{start, end}
}

func titleFor(conv chatgpt.Conversation) string {
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = "(untitled conversation)"
	}

	stamps := make([]string, 0, 2)
	if !conv.UpdateTime.IsZero() {
		stamps = append(stamps, conv.UpdateTime.Local().Format("2006-01-02"))
	}
	if conv.IsArchived {
		stamps = append(stamps, "archived")
	}
	if len(stamps) == 0 {
		return fmt.Sprintf("%s [%s]", title, shortID(conv.ID))
	}
	return fmt.Sprintf("%s [%s] %s", title, shortID(conv.ID), strings.Join(stamps, " • "))
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func loadConversationsCmd(client *chatgpt.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		conversations, err := client.ListConversations(ctx, 100)
		if err != nil {
			return loadResultMsg{err: err}
		}

		slices.SortStableFunc(conversations, func(a, b chatgpt.Conversation) int {
			if a.UpdateTime.Equal(b.UpdateTime.Time) {
				return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
			}
			if a.UpdateTime.After(b.UpdateTime.Time) {
				return -1
			}
			return 1
		})

		return loadResultMsg{conversations: conversations}
	}
}

func pollStatusCmd(client *chatgpt.Client) tea.Cmd {
	return tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
		return statusMsg(client.Status())
	})
}

func runBulkActionCmd(client *chatgpt.Client, conversations []chatgpt.Conversation, action actionChoice) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		results := make([]actionResult, 0, len(conversations))
		for _, conv := range conversations {
			var err error
			switch action {
			case actionArchive:
				err = client.ArchiveConversation(ctx, conv.ID)
			case actionDelete:
				err = client.DeleteConversation(ctx, conv.ID)
			}
			results = append(results, actionResult{
				id:    conv.ID,
				label: titleFor(conv),
				err:   err,
			})
		}
		return actionFinishedMsg{results: results}
	}
}

var (
	docStyle          = lipgloss.NewStyle().Padding(1, 2)
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	subtleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selectedLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	warningStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	successStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
)
