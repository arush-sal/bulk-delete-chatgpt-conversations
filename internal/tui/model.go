package tui

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/arush-sal/bulk-delete-chatgpt-conversations/internal/chatgpt"
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

type snapshotMsg struct {
	status    string
	logs      []string
	email     string
	sessionID string
}

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
	version       string
	width         int
	height        int
	phase         phase
	loadingText   string
	logs          []string
	email         string
	sessionID     string
	conversations []chatgpt.Conversation
	cursor        int
	selected      map[string]struct{}
	actionCursor  int
	runningIndex  int
	results       []actionResult
	err           error
}

func New(client *chatgpt.Client, version string) Model {
	return Model{
		client:      client,
		version:     version,
		phase:       phaseLoading,
		loadingText: "Launching Chrome and loading your ChatGPT session...\n\nIf Chrome opens a login page, finish signing in there and keep this terminal open.",
		sessionID:   client.SessionIDLabel(),
		selected:    make(map[string]struct{}),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadConversationsCmd(m.client), pollSnapshotCmd(m.client))
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
	case snapshotMsg:
		m.logs = msg.logs
		m.email = msg.email
		m.sessionID = msg.sessionID
		if text := strings.TrimSpace(msg.status); text != "" {
			m.loadingText = text
		}
		if m.phase != phaseDone && m.phase != phaseError {
			return m, pollSnapshotCmd(m.client)
		}
	case actionFinishedMsg:
		m.results = msg.results
		m.phase = phaseDone
		return m, nil
	case tea.KeyPressMsg:
		switch m.phase {
		case phaseLoading, phaseRunning:
			if isQuitKey(msg) {
				return m, tea.Quit
			}
		case phaseSelect:
			return m.updateSelection(msg)
		case phaseAction:
			return m.updateActionPicker(msg)
		case phaseConfirm:
			return m.updateConfirmation(msg)
		case phaseDone, phaseError:
			if isConfirmKey(msg) || isQuitKey(msg) || isBackKey(msg) {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	var content string
	switch m.phase {
	case phaseLoading:
		content = m.renderLoading()
	case phaseSelect:
		content = m.renderSelection()
	case phaseAction:
		content = m.renderActionPicker()
	case phaseConfirm:
		content = m.renderConfirmation()
	case phaseRunning:
		content = m.renderRunning()
	case phaseDone:
		content = m.renderDone()
	case phaseError:
		content = m.renderError()
	default:
		content = ""
	}
	v := tea.NewView(baseAppStyle.Width(max(80, m.width)).Height(max(24, m.height)).Render(content))
	v.AltScreen = true
	return v
}

func (m Model) updateSelection(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case isQuitKey(msg):
		return m, tea.Quit
	case isUpKey(msg):
		if m.cursor > 0 {
			m.cursor--
		}
	case isDownKey(msg):
		if m.cursor < len(m.conversations)-1 {
			m.cursor++
		}
	case isToggleKey(msg):
		m.toggleCurrent()
	case matchesRune(msg, "a"):
		if len(m.selected) == len(m.conversations) {
			m.selected = make(map[string]struct{})
		} else {
			for _, conv := range m.conversations {
				m.selected[conv.ID] = struct{}{}
			}
		}
	case isConfirmKey(msg):
		if len(m.selected) > 0 {
			m.phase = phaseAction
		}
	}
	return m, nil
}

func (m Model) updateActionPicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case isQuitKey(msg):
		return m, tea.Quit
	case isUpKey(msg):
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case isDownKey(msg):
		if m.actionCursor < len(actionLabels)-1 {
			m.actionCursor++
		}
	case isBackKey(msg):
		m.phase = phaseSelect
	case isConfirmKey(msg):
		if m.actionCursor == int(actionCancel) {
			m.phase = phaseSelect
			return m, nil
		}
		m.phase = phaseConfirm
	}
	return m, nil
}

func (m Model) updateConfirmation(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case isQuitKey(msg):
		return m, tea.Quit
	case matchesRune(msg, "n") || isBackKey(msg):
		m.phase = phaseAction
	case matchesRune(msg, "y") || isConfirmKey(msg):
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
	if len(m.conversations) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderChrome(),
			"",
			m.renderPanel("Conversations", statusPanelStyle.Width(m.contentWidth()).Render("No conversations found for this account.")),
			"",
			m.renderFooterBar("q quit"),
		)
	}

	tableHeight, _ := m.bodyHeights()
	table := m.renderPanel(
		fmt.Sprintf("conversations(all)[%d]", len(m.conversations)),
		lipgloss.JoinVertical(
			lipgloss.Left,
			tableMetaStyle.Render(fmt.Sprintf("%d loaded   %d selected   %s", len(m.conversations), len(m.selected), m.phaseLabel())),
			m.renderConversationTable(tableHeight),
		),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderChrome(),
		"",
		table,
		"",
		m.renderFooterBar("j/k move   space toggle   a all/none   enter continue   q quit"),
	)
}

func (m Model) renderActionPicker() string {
	var options strings.Builder
	options.WriteString(titleStyle.Render("Choose Action"))
	options.WriteString("\n")
	options.WriteString(subtleStyle.Render(fmt.Sprintf("%d conversations selected", len(m.selected))))
	options.WriteString("\n\n")

	for i, label := range actionLabels {
		prefix := "  "
		if i == m.actionCursor {
			prefix = "> "
			options.WriteString(selectedLineStyle.Render(prefix + label))
		} else {
			options.WriteString(prefix + label)
		}
		options.WriteString("\n")
	}

	options.WriteString("\n")
	options.WriteString(helpStyle.Render("up/down: move  enter: choose  esc: back"))
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderChrome(),
		"",
		m.renderPanel("actions", statusPanelStyle.Width(m.contentWidth()).Render(options.String())),
		"",
		m.renderFooterBar("j/k move   enter choose   esc back"),
	)
}

func (m Model) renderConfirmation() string {
	actionLabel := actionLabels[m.actionCursor]
	targets := m.selectedConversations()

	var panel strings.Builder
	panel.WriteString(titleStyle.Render("Confirm"))
	panel.WriteString("\n")
	panel.WriteString(fmt.Sprintf("%s %d conversations?\n\n", actionLabel, len(targets)))
	for i, conv := range targets {
		if i >= 8 {
			panel.WriteString(subtleStyle.Render(fmt.Sprintf("...and %d more", len(targets)-i)))
			panel.WriteString("\n")
			break
		}
		panel.WriteString("• " + displayTitle(conv) + "\n")
	}
	panel.WriteString("\n")
	if m.selectedAction() == actionDelete {
		panel.WriteString(warningStyle.Render("Delete marks conversations as not visible and may be hard to recover."))
	} else {
		panel.WriteString(subtleStyle.Render("Archive hides selected conversations without deleting them."))
	}
	panel.WriteString("\n\n")
	panel.WriteString(helpStyle.Render("y/enter: confirm  n/esc: back"))
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderChrome(),
		"",
		m.renderPanel("confirm", statusPanelStyle.Width(m.contentWidth()).Render(panel.String())),
		"",
		m.renderFooterBar("y confirm   n back"),
	)
}

func (m Model) renderRunning() string {
	_, logHeight := m.loadingHeights()
	panel := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Processing"),
		"",
		fmt.Sprintf("Applying %s to %d conversations...", strings.ToLower(actionLabels[m.actionCursor]), len(m.selected)),
		subtleStyle.Render("Please wait."),
		"",
		logViewportStyle.Width(m.contentWidth()).Height(logHeight+2).Render(strings.Join(m.tailLogs(m.contentWidth()-4), "\n")),
	)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderChrome(),
		"",
		m.renderPanel("processing", statusPanelStyle.Width(m.contentWidth()).Render(panel)),
		"",
		m.renderFooterBar("q quit"),
	)
}

func (m Model) renderDone() string {
	if len(m.results) == 0 && len(m.conversations) == 0 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderChrome(),
			"",
			m.renderPanel("completed", statusPanelStyle.Width(m.contentWidth()).Render("No conversations found.")),
			"",
			m.renderFooterBar("q quit"),
		)
	}

	successes := 0
	for _, result := range m.results {
		if result.err == nil {
			successes++
		}
	}

	var panel strings.Builder
	panel.WriteString(titleStyle.Render("Completed"))
	panel.WriteString("\n")
	panel.WriteString(subtleStyle.Render(fmt.Sprintf("%d succeeded, %d failed", successes, len(m.results)-successes)))
	panel.WriteString("\n\n")
	for _, result := range m.results {
		status := successStyle.Render("OK")
		if result.err != nil {
			status = errorStyle.Render("FAIL")
		}
		panel.WriteString(fmt.Sprintf("[%s] %s (%s)", status, result.label, shortID(result.id)))
		if result.err != nil {
			panel.WriteString("\n    " + subtleStyle.Render(result.err.Error()))
		}
		panel.WriteString("\n")
	}
	panel.WriteString("\n")
	panel.WriteString(helpStyle.Render("Press enter or q to exit."))
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderChrome(),
		"",
		m.renderPanel("completed", statusPanelStyle.Width(m.contentWidth()).Render(panel.String())),
		"",
		m.renderFooterBar("enter exit   q quit"),
	)
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

func (m Model) renderLoading() string {
	mainHeight, _ := m.loadingHeights()
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		statusBannerStyle.Width(m.contentWidth()).Render(m.loadingText),
		"",
		logViewportStyle.Width(m.contentWidth()).Height(mainHeight).Render(strings.Join(m.tailLogs(m.contentWidth()-4), "\n")),
	)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderChrome(),
		"",
		m.renderPanel("auth(session)", body),
		"",
		m.renderFooterBar("q quit"),
	)
}

func (m Model) renderError() string {
	_, logHeight := m.loadingHeights()
	errBody := lipgloss.JoinVertical(
		lipgloss.Left,
		statusPanelStyle.BorderForeground(lipgloss.Color("#FF7B72")).Width(m.contentWidth()).Render(renderError(m.err)),
		"",
		logViewportStyle.Width(m.contentWidth()).Height(logHeight).Render(strings.Join(m.tailLogs(m.contentWidth()-4), "\n")),
	)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderChrome(),
		"",
		m.renderPanel("error", errBody),
		"",
		m.renderFooterBar("enter exit   q quit"),
	)
}

func (m Model) renderChrome() string {
	top := lipgloss.JoinHorizontal(
		lipgloss.Top,
		appTitleStyle.Render("bulk-delete-chatgpt-conversations"),
		"  ",
		phaseBadgeStyle.Render(m.phaseLabel()),
	)
	info := []string{
		metaKeyStyle.Render("Email: ") + metaValueStyle.Render(valueOrPlaceholder(m.email)),
		metaKeyStyle.Render("Session: ") + metaValueStyle.Render(valueOrPlaceholder(m.sessionID)),
		metaKeyStyle.Render("Version: ") + metaValueStyle.Render(valueOrPlaceholder(m.version)),
		metaKeyStyle.Render("Go: ") + metaValueStyle.Render(runtime.Version()),
	}
	rowOne := compactBarStyle.Width(m.contentWidth()).Render(strings.Join(info[:2], "   "))
	rowTwo := compactBarStyle.Width(m.contentWidth()).Render(strings.Join(info[2:], "   "))
	return lipgloss.JoinVertical(lipgloss.Left, top, rowOne, rowTwo)
}

func (m Model) renderConversationTable(height int) string {
	visible := m.visibleRange()
	headers := []string{"Select", "Conversation Title", "Date", "Age"}
	contentWidth := m.contentWidth() - 2
	widths := []int{10, max(24, contentWidth-38), 14, 8}

	var rows []string
	rows = append(rows, tableHeaderStyle.Render(
		formatTableRow(widths, headers[0], headers[1], headers[2], headers[3]),
	))

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
		date := ""
		if !conv.UpdateTime.IsZero() {
			date = conv.UpdateTime.Local().Format("2006-01-02")
		}
		selector := cursor + " [" + mark + "]"
		row := formatTableRow(
			widths,
			selector,
			trim(displayTitle(conv), widths[1]),
			date,
			ageString(conv.UpdateTime.Time),
		)
		if i == m.cursor {
			rows = append(rows, selectedRowStyle.Render(row))
		} else {
			rows = append(rows, tableRowStyle.Render(row))
		}
	}

	tableBody := strings.Join(rows, "\n")
	return tableBoxStyle.Height(height).Width(contentWidth).Render(tableBody)
}

func trim(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func pad(value string, width int) string {
	if width <= 0 {
		return value
	}
	return lipgloss.NewStyle().Width(width).Render(value)
}

func ageString(updated time.Time) string {
	if updated.IsZero() {
		return "-"
	}
	d := time.Since(updated)
	switch {
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dyr", int(d.Hours()/(24*365)))
	}
}

func valueOrPlaceholder(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Waiting..."
	}
	return value
}

func (m Model) tailLogs(width int) []string {
	if width <= 0 {
		width = 120
	}
	wrapped := make([]string, 0, len(m.logs))
	for _, line := range m.logs {
		wrapped = append(wrapped, wrapLines(line, width)...)
	}
	if len(wrapped) <= 8 {
		return wrapped
	}
	return wrapped[len(wrapped)-8:]
}

func (m Model) visibleRange() struct{ start, end int } {
	tableHeight, _ := m.bodyHeights()
	maxItems := max(8, tableHeight-1)
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

func displayTitle(conv chatgpt.Conversation) string {
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = "(untitled conversation)"
	}
	if conv.IsArchived {
		return title + " [archived]"
	}
	return title
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

func pollSnapshotCmd(client *chatgpt.Client) tea.Cmd {
	return tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
		return snapshotMsg{
			status:    client.Status(),
			logs:      client.Logs(),
			email:     client.UserEmail(),
			sessionID: client.SessionIDLabel(),
		}
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
	baseAppStyle      = lipgloss.NewStyle().Padding(1, 1).Background(lipgloss.Color("#1A1826")).Foreground(lipgloss.Color("#D8D6EA"))
	appTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4B942"))
	phaseBadgeStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1A1826")).Background(lipgloss.Color("#5FD7FF")).Padding(0, 1)
	metaKeyStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4B942"))
	metaValueStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8D6EA"))
	compactBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8D6EA"))
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FD7FF"))
	subtleStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A88A8"))
	selectedLineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7FFFD4")).Bold(true)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A3C2"))
	warningStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#F7C95C"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Bold(true)
	successStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#6AE3A8")).Bold(true)
	cardTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FD7FF"))
	statusPanelStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5FD7FF")).Padding(1, 2)
	statusBannerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6AE3A8")).Bold(true)
	logViewportStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8D6EA"))
	panelStyle        = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#7FDBFF")).Padding(0, 1)
	panelTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FD7FF"))
	tableMetaStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A88A8"))
	tableBoxStyle     = lipgloss.NewStyle()
	tableHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F3F0FF"))
	tableRowStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BD5FF"))
	selectedRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#1A1826")).Background(lipgloss.Color("#C084FC")).Bold(true)
	footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#1A1826")).Background(lipgloss.Color("#F4B942")).Padding(0, 1)
)

func (m Model) bodyHeights() (int, int) {
	if m.height <= 0 {
		return 12, 8
	}
	headerHeight := lipgloss.Height(m.renderChrome())
	available := m.height - headerHeight - 8
	logHeight := min(8, max(6, available/3))
	tableHeight := max(10, available-logHeight)
	return tableHeight, logHeight
}

func (m Model) loadingHeights() (int, int) {
	if m.height <= 0 {
		return 10, 8
	}
	headerHeight := lipgloss.Height(m.renderChrome())
	available := m.height - headerHeight - 8
	logHeight := min(8, max(6, available/3))
	mainHeight := max(10, available-logHeight-2)
	return mainHeight, logHeight
}

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 100
	}
	return max(72, m.width-4)
}

func (m Model) renderPanel(title, body string) string {
	return panelStyle.Width(m.contentWidth()).Render(
		lipgloss.JoinVertical(lipgloss.Left, panelTitleStyle.Render(title), body),
	)
}

func (m Model) renderFooterBar(text string) string {
	return footerStyle.Width(m.contentWidth()).Render(text)
}

func (m Model) phaseLabel() string {
	switch m.phase {
	case phaseLoading:
		return "loading"
	case phaseSelect:
		return "select"
	case phaseAction:
		return "action"
	case phaseConfirm:
		return "confirm"
	case phaseRunning:
		return "running"
	case phaseDone:
		return "done"
	case phaseError:
		return "error"
	default:
		return "idle"
	}
}

func formatTableRow(widths []int, values ...string) string {
	parts := make([]string, 0, len(values))
	for i, value := range values {
		parts = append(parts, pad(value, widths[i]))
	}
	return strings.Join(parts, " ")
}

func wrapLines(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	if len(runes) <= width {
		return []string{s}
	}
	lines := make([]string, 0, (len(runes)/width)+1)
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		lines = append(lines, string(runes))
	}
	return lines
}

func isQuitKey(msg tea.KeyPressMsg) bool {
	return msg.String() == "ctrl+c" || msg.String() == "q"
}

func isBackKey(msg tea.KeyPressMsg) bool {
	return msg.Code == tea.KeyEscape || msg.String() == "esc"
}

func isConfirmKey(msg tea.KeyPressMsg) bool {
	return msg.Code == tea.KeyEnter || msg.Code == tea.KeyReturn || msg.String() == "enter"
}

func isUpKey(msg tea.KeyPressMsg) bool {
	return msg.Code == tea.KeyUp || msg.String() == "k" || msg.String() == "up"
}

func isDownKey(msg tea.KeyPressMsg) bool {
	return msg.Code == tea.KeyDown || msg.String() == "j" || msg.String() == "down"
}

func isToggleKey(msg tea.KeyPressMsg) bool {
	return msg.Code == tea.KeySpace || msg.String() == " " || msg.String() == "space"
}

func matchesRune(msg tea.KeyPressMsg, want string) bool {
	return strings.EqualFold(msg.String(), want)
}
