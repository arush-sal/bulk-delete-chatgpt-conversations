package tui

import (
	"context"
	"fmt"
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
	refresh       bool
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

type sortMode int

const (
	sortDateDesc sortMode = iota
	sortDateAsc
	sortTitleAsc
	sortTitleDesc
)

var sortLabels = []string{"newest", "oldest", "A-Z", "Z-A"}

const appBg = lipgloss.Color("#1A1826")

var (
	baseAppStyle       = lipgloss.NewStyle().PaddingTop(1).PaddingLeft(1).PaddingRight(1).PaddingBottom(0).Background(appBg).Foreground(lipgloss.Color("#D8D6EA"))
	appTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4B942")).Background(appBg)
	headerBoxStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5FFF8F")).Padding(0, 1).Background(appBg)
	headerMetaStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A3C2")).Background(appBg)
	summaryCardStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5A5878")).Padding(0, 1).Background(appBg)
	summaryTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8A88A8")).Background(appBg)
	summaryValueStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F3F0FF")).Background(appBg)
	summarySubtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A3C2")).Background(appBg)
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FFF8F")).Background(appBg)
	subtleStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A88A8")).Background(appBg)
	selectedLineStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7FFFD4")).Bold(true).Background(appBg)
	helpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A3C2")).Background(appBg)
	warningStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F7C95C")).Background(appBg)
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Bold(true).Background(appBg)
	successStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6AE3A8")).Bold(true).Background(appBg)
	statusPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5FD7FF")).Padding(1, 2).Background(appBg)
	errorBoxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#FF7B72")).Padding(1, 2).Background(appBg)
	statusBannerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6AE3A8")).Bold(true).Background(appBg)
	logViewportStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8D6EA")).Background(appBg)
	panelStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5FD7FF")).Padding(0, 1).Background(appBg)
	panelTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FD7FF")).Background(appBg)
	tableMetaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A88A8")).Background(appBg)
	tableBoxStyle      = lipgloss.NewStyle().Background(appBg)
	tableHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F3F0FF")).Background(appBg)
	tableRowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BD5FF")).Background(appBg)
	selectedRowStyle   = lipgloss.NewStyle().Foreground(appBg).Background(lipgloss.Color("#C084FC")).Bold(true)
	filterStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A88A8")).Background(appBg)
	filterActiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4B942")).Bold(true).Background(appBg)
	sidebarKeyStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4B942")).Background(appBg)
	sidebarDescStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A3C2")).Background(appBg)
	actionBarStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5A5878")).Padding(0, 1).Background(appBg)
	actionIdleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A3C2")).Background(appBg)
	actionHotStyle     = lipgloss.NewStyle().Bold(true).Foreground(appBg).Background(lipgloss.Color("#5FFF8F")).Padding(0, 1)
	actionDangerStyle  = lipgloss.NewStyle().Bold(true).Foreground(appBg).Background(lipgloss.Color("#FF7B72")).Padding(0, 1)
	actionMuteStyle    = lipgloss.NewStyle().Bold(true).Foreground(appBg).Background(lipgloss.Color("#8A88A8")).Padding(0, 1)
)

// Model owns the full TUI state machine: auth/loading, selection, confirmation,
// bulk execution, and the final result screen.
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
	conversations []chatgpt.Conversation // original full list
	filtered      []chatgpt.Conversation // filtered + sorted view
	cursor        int
	selected      map[string]struct{}
	actionCursor  int
	runningIndex  int
	results       []actionResult
	err           error
	filterText    string
	filtering     bool // true when filter input is active
	sortBy        sortMode
}

func New(client *chatgpt.Client, version string) Model {
	return Model{
		client:      client,
		version:     version,
		phase:       phaseLoading,
		loadingText: "",
		sessionID:   client.SessionIDLabel(),
		selected:    make(map[string]struct{}),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadCachedConversationsCmd(m.client), loadConversationsCmd(m.client), pollSnapshotCmd(m.client))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case loadResultMsg:
		if msg.err != nil {
			if msg.refresh && len(m.conversations) == 0 {
				m.phase = phaseError
				m.err = msg.err
			}
			return m, nil
		}
		if !msg.refresh && len(msg.conversations) == 0 {
			return m, nil
		}

		m.conversations = msg.conversations
		m.applyFilterAndSort()
		m.phase = phaseSelect
		if msg.refresh && len(m.conversations) == 0 {
			m.phase = phaseDone
		}
	case snapshotMsg:
		// Status/log snapshots come from the ChatGPT client while browser auth
		// and API pagination are happening in the background.
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
		m.reconcileActionResults(msg.results)
		if len(m.conversations) == 0 {
			m.phase = phaseDone
		} else {
			m.phase = phaseSelect
		}
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
			if isQuitKey(msg) || (m.phase == phaseError && (isConfirmKey(msg) || isBackKey(msg))) {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	w := max(80, m.width)
	h := max(24, m.height)

	// 1. Render header (fixed)
	header := m.renderChrome()

	// 3. Render body content (fills remaining space). Reserve one extra row so
	// the bottom border of the main panel is not clipped by terminals that
	// render padding and borders a little differently.
	headerH := lipgloss.Height(header)
	bodyH := h - headerH - 2
	if bodyH < 0 {
		bodyH = 0
	}

	var body string
	switch m.phase {
	case phaseLoading:
		body = m.renderLoading(bodyH)
	case phaseSelect:
		body = m.renderSelection(bodyH)
	case phaseAction:
		body = m.renderActionPicker(bodyH)
	case phaseConfirm:
		body = m.renderConfirmation(bodyH)
	case phaseRunning:
		body = m.renderRunning(bodyH)
	case phaseDone:
		body = m.renderDone(bodyH)
	case phaseError:
		body = m.renderError(bodyH)
	}

	// The active body view owns its height so the header can absorb shortcuts
	// and the main panel can consume the remaining terminal space.
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
	)

	// Height(h-1): content area excludes the 1-row top padding.
	v := tea.NewView(baseAppStyle.Width(w).Height(h - 1).Render(content))
	v.AltScreen = true
	return v
}

func (m Model) shortcutHints() string {
	switch m.phase {
	case phaseLoading:
		return "q quit"
	case phaseSelect:
		if m.filtering {
			return "type filter   enter done   esc cancel"
		}
		return "j/k move   pgup/pgdn page   space mark   a all   / filter   s sort   enter actions   q quit"
	case phaseAction:
		return "j/k move   enter choose   esc back"
	case phaseConfirm:
		return "y confirm   esc back"
	case phaseRunning:
		return "q quit"
	case phaseDone:
		return "q quit"
	case phaseError:
		return "enter exit   q quit"
	default:
		return "q quit"
	}
}

func (m Model) updateSelection(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// When filtering is active, handle text input
	if m.filtering {
		switch {
		case isBackKey(msg):
			m.filtering = false
		case isConfirmKey(msg):
			m.filtering = false
		case msg.Code == tea.KeyBackspace || msg.String() == "backspace":
			if len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
				m.applyFilterAndSort()
				m.cursor = 0
			}
		default:
			r := msg.String()
			if len(r) == 1 || r == " " {
				m.filterText += r
				m.applyFilterAndSort()
				m.cursor = 0
			}
		}
		return m, nil
	}

	switch {
	case isQuitKey(msg):
		return m, tea.Quit
	case matchesRune(msg, "/"):
		m.filtering = true
	case matchesRune(msg, "s"):
		m.sortBy = (m.sortBy + 1) % 4
		m.applyFilterAndSort()
		m.cursor = 0
	case isUpKey(msg):
		if m.cursor > 0 {
			m.cursor--
		}
	case isDownKey(msg):
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case isPageUpKey(msg):
		m.cursor -= m.pageSize()
		m.clampCursor()
	case isPageDownKey(msg):
		m.cursor += m.pageSize()
		m.clampCursor()
	case isToggleKey(msg):
		m.toggleCurrent()
	case matchesRune(msg, "a"):
		if len(m.selected) == len(m.filtered) {
			m.selected = make(map[string]struct{})
		} else {
			for _, conv := range m.filtered {
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
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	m.clampCursor()
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return
	}
	id := m.filtered[m.cursor].ID
	if _, ok := m.selected[id]; ok {
		delete(m.selected, id)
		return
	}
	m.selected[id] = struct{}{}
}

func (m *Model) applyFilterAndSort() {
	// Filter
	if m.filterText == "" {
		m.filtered = make([]chatgpt.Conversation, len(m.conversations))
		copy(m.filtered, m.conversations)
	} else {
		m.filtered = m.filtered[:0]
		needle := strings.ToLower(m.filterText)
		for _, conv := range m.conversations {
			if strings.Contains(strings.ToLower(conv.Title), needle) {
				m.filtered = append(m.filtered, conv)
			}
		}
	}

	// Sort
	slices.SortStableFunc(m.filtered, func(a, b chatgpt.Conversation) int {
		switch m.sortBy {
		case sortDateAsc:
			if a.UpdateTime.Equal(b.UpdateTime.Time) {
				return 0
			}
			if a.UpdateTime.Before(b.UpdateTime.Time) {
				return -1
			}
			return 1
		case sortTitleAsc:
			return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		case sortTitleDesc:
			return strings.Compare(strings.ToLower(b.Title), strings.ToLower(a.Title))
		default: // sortDateDesc
			if a.UpdateTime.Equal(b.UpdateTime.Time) {
				return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
			}
			if a.UpdateTime.After(b.UpdateTime.Time) {
				return -1
			}
			return 1
		}
	})

	// Clamp cursor
	m.clampCursor()
}

func (m *Model) clampCursor() {
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
}

func (m *Model) reconcileActionResults(results []actionResult) {
	if len(results) == 0 {
		m.selected = make(map[string]struct{})
		m.actionCursor = 0
		m.applyFilterAndSort()
		return
	}

	successful := make(map[string]struct{}, len(results))
	for _, result := range results {
		if result.err == nil {
			successful[result.id] = struct{}{}
			delete(m.selected, result.id)
		}
	}

	if len(successful) > 0 {
		next := m.conversations[:0]
		for _, conv := range m.conversations {
			if _, ok := successful[conv.ID]; ok {
				continue
			}
			next = append(next, conv)
		}
		m.conversations = next
	}

	m.actionCursor = 0
	m.applyFilterAndSort()
}

func (m Model) selectedAction() actionChoice {
	return actionChoice(m.actionCursor)
}

// selectedConversations preserves the on-screen ordering instead of returning
// map iteration order, which keeps confirmations and action results stable.
func (m Model) selectedConversations() []chatgpt.Conversation {
	selected := make([]chatgpt.Conversation, 0, len(m.selected))
	for _, conv := range m.conversations {
		if _, ok := m.selected[conv.ID]; ok {
			selected = append(selected, conv)
		}
	}
	return selected
}

func (m Model) renderSelection(bodyH int) string {
	return m.renderWorkspaceLayout(bodyH, m.renderConversationListPane)
}

func (m Model) renderActionPicker(bodyH int) string {
	return m.renderWorkspaceLayout(bodyH, m.renderConversationListPane)
}

func (m Model) renderConfirmation(bodyH int) string {
	return m.renderWorkspaceLayout(bodyH, m.renderConversationListPane)
}

func (m Model) renderRunning(bodyH int) string {
	logHeight := max(4, bodyH-10)
	panel := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Processing"),
		"",
		fmt.Sprintf("Applying %s to %d conversations...", strings.ToLower(actionLabels[m.actionCursor]), len(m.selected)),
		subtleStyle.Render("Please wait."),
		"",
		logViewportStyle.Width(m.contentWidth()).Height(logHeight).Render(strings.Join(m.tailLogs(m.contentWidth()-4), "\n")),
	)
	return m.renderPanelSized("Processing", statusPanelStyle.Width(m.contentWidth()).Render(panel), bodyH)
}

func (m Model) renderDone(bodyH int) string {
	if len(m.results) == 0 && len(m.conversations) == 0 {
		return m.renderPanelSized("Completed", statusPanelStyle.Width(m.contentWidth()).Render("No conversations found."), bodyH)
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
	return m.renderPanelSized("Completed", statusPanelStyle.Width(m.contentWidth()).Render(panel.String()), bodyH)
}

func renderErrorText(err error) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Error"))
	b.WriteString("\n\n")
	if err != nil {
		b.WriteString(errorStyle.Render(err.Error()))
		b.WriteString("\n\n")
	}
	b.WriteString(subtleStyle.Render("Check your internet connection, and try again."))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("Press enter or q to exit."))
	return b.String()
}

func (m Model) renderLoading(bodyH int) string {
	logHeight := max(4, bodyH-6)
	separator := lipgloss.NewStyle().Background(appBg).Width(max(1, m.contentWidth()-4)).Render("")
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		statusBannerStyle.Render(m.loadingText),
		separator,
		logViewportStyle.Width(m.contentWidth()).Height(logHeight).Render(strings.Join(m.tailLogs(m.contentWidth()-4), "\n")),
	)
	return m.renderPanelSized("Auth", body, bodyH)
}

func (m Model) renderError(bodyH int) string {
	var body strings.Builder
	body.WriteString(errorBoxStyle.Width(m.contentWidth()).Render(renderErrorText(m.err)))

	logs := strings.Join(m.tailLogs(m.contentWidth()-4), "\n")
	if strings.TrimSpace(logs) != "" {
		body.WriteString("\n\n")
		body.WriteString(subtleStyle.Render("Recent logs"))
		body.WriteString("\n")
		body.WriteString(logViewportStyle.Width(m.contentWidth()).Render(logs))
	}

	return m.renderPanelSized("Error", body.String(), bodyH)
}

func (m Model) renderChrome() string {
	w := m.contentWidth()
	innerW := innerWidthForStyle(headerBoxStyle, w)
	titleRow := renderBoundedText(appTitleStyle, "chatgpt-bulk  "+valueOrPlaceholder(m.version), innerW)

	cardWidths := splitFixedWidth(innerW-2, 4, 1)
	cards := []string{
		m.renderSummaryCard(cardWidths[0], "SESSION", valueOrPlaceholder(m.email), valueOrPlaceholder(m.sessionID)),
		m.renderSummaryCard(cardWidths[1], "MODE", m.modeSummary(), m.modeSummaryDetail()),
		m.renderSummaryCard(cardWidths[2], "CACHE", m.cacheSummary(), m.cacheSummaryDetail()),
		m.renderSummaryCard(cardWidths[3], "SELECTION", m.selectionSummary(), m.selectionSummaryDetail()),
	}
	cardParts := make([]string, 0, len(cards)*2)
	for i, card := range cards {
		if i > 0 {
			cardParts = append(cardParts, " ")
		}
		cardParts = append(cardParts, card)
	}
	cardRow := lipgloss.JoinHorizontal(lipgloss.Top, cardParts...)

	return headerBoxStyle.Width(innerW).Render(lipgloss.JoinVertical(lipgloss.Left, titleRow, "", cardRow))
}

func (m Model) renderConversationTable(height int) string {
	visible := m.visibleRange(height)
	headers := []string{"", "Conversation Title", "Updated", "State"}
	contentWidth := max(1, m.leftPanelInnerWidth()-1)
	widths := m.conversationColumnWidths(contentWidth)

	var rows []string
	rows = append(rows, tableHeaderStyle.Render(
		formatConversationRow(widths, headers[0], headers[1], headers[2], headers[3]),
	))

	// Only render the visible window around the cursor so the pane height stays
	// stable even when the account has hundreds of conversations.
	for i := visible.start; i < visible.end; i++ {
		conv := m.filtered[i]
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		mark := " "
		if _, ok := m.selected[conv.ID]; ok {
			mark = "x"
		}
		date := "-"
		if !conv.UpdateTime.IsZero() {
			date = conv.UpdateTime.Local().Format("Jan 02 15:04")
		}
		state := "active"
		if conv.IsArchived {
			state = "archived"
		}
		selector := cursor + "[" + mark + "]"
		row := formatConversationRow(
			widths,
			selector,
			trim(displayTitle(conv), widths[1]),
			date,
			state,
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

func (m Model) renderWorkspaceLayout(bodyH int, leftRenderer func(int) string) string {
	actionBarH := outerHeightForStyle(actionBarStyle, 1)
	workspaceH := max(8, bodyH-actionBarH-2)
	_, rightW := m.workspaceColumnWidths()

	left := leftRenderer(workspaceH)
	right := m.renderSidebar(workspaceH, rightW)
	workspace := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	actionBar := m.renderActionBar()
	note := subtleStyle.Render("Real browser auth first. Terminal cleanup second.")
	return lipgloss.JoinVertical(lipgloss.Left, workspace, "", actionBar, note)
}

func (m Model) renderConversationListPane(height int) string {
	if len(m.conversations) == 0 {
		return m.renderPanelWithWidth("Conversations", statusPanelStyle.Width(m.leftPanelInnerWidth()).Render("No conversations found for this account."), height, m.leftPanelWidth())
	}

	bodyLines := []string{
		m.renderFilterLine(),
		m.renderConversationTable(m.selectionTableHeight(height)),
		tableMetaStyle.Render(m.selectionFooterLine()),
	}
	return m.renderPanelWithWidth("Conversations", lipgloss.JoinVertical(lipgloss.Left, bodyLines...), height, m.leftPanelWidth())
}

func (m Model) renderSidebar(height int, width int) string {
	topH, middleH, bottomH := m.sidebarPanelHeights(height)

	nextAction := m.renderPanelWithWidth("Next Action", m.renderNextActionPanel(), topH, width)
	shortcuts := m.renderPanelWithWidth("keyboard first", m.renderShortcutsPanel(), middleH, width)
	status := m.renderPanelWithWidth("Status Log", m.renderStatusLogPanel(bottomH), bottomH, width)
	return lipgloss.JoinVertical(lipgloss.Left, nextAction, "", shortcuts, "", status)
}

func (m Model) renderNextActionPanel() string {
	label, detail := m.nextActionSummary()
	lines := []string{
		titleStyle.Render(label),
		summarySubtleStyle.Render(detail),
	}

	if m.phase == phaseAction {
		lines = append(lines, "")
		for i, actionLabel := range actionLabels {
			line := "  " + actionLabel
			if i == m.actionCursor {
				line = "> " + actionLabel
				lines = append(lines, selectedLineStyle.Render(line))
				continue
			}
			lines = append(lines, subtleStyle.Render(line))
		}
	}

	if m.phase == phaseConfirm {
		lines = append(lines, "")
		if m.selectedAction() == actionDelete {
			lines = append(lines, warningStyle.Render("Delete keeps the current behavior and may be hard to recover."))
		} else {
			lines = append(lines, subtleStyle.Render("Archive keeps the current behavior and hides the selected rows."))
		}
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderShortcutsPanel() string {
	rows := []struct {
		key  string
		desc string
	}{
		{"/", "list / filter"},
		{"space", "toggle row"},
		{"a", "select all"},
		{"s", "sort"},
		{"esc", "cancel"},
		{"q", "quit"},
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, pad(sidebarKeyStyle.Render(row.key), 8)+sidebarDescStyle.Render(row.desc))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderStatusLogPanel(height int) string {
	lines := m.tailLogs(m.rightPanelInnerWidth() - 1)
	if len(lines) == 0 {
		lines = []string{"Waiting for events..."}
	}
	return logViewportStyle.Width(m.rightPanelInnerWidth()).Height(max(1, height-panelBodyFrameHeight())).Render(strings.Join(lines, "\n"))
}

func (m Model) renderActionBar() string {
	archive := actionIdleStyle.Render("archive selected")
	deleteAction := actionIdleStyle.Render("delete selected")
	cancel := actionIdleStyle.Render("cancel")

	switch m.phase {
	case phaseAction:
		switch m.selectedAction() {
		case actionArchive:
			archive = actionHotStyle.Render("archive selected")
		case actionDelete:
			deleteAction = actionDangerStyle.Render("delete selected")
		case actionCancel:
			cancel = actionMuteStyle.Render("cancel")
		}
	case phaseConfirm:
		if m.selectedAction() == actionDelete {
			deleteAction = actionDangerStyle.Render("delete selected")
		} else {
			archive = actionHotStyle.Render("archive selected")
		}
	default:
		if len(m.selected) > 0 {
			archive = actionHotStyle.Render("archive selected")
			deleteAction = actionDangerStyle.Render("delete selected")
		}
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, archive, "   ", deleteAction, "   ", cancel)
	return actionBarStyle.Width(innerWidthForStyle(actionBarStyle, m.contentWidth())).Render(content)
}

func (m Model) renderSummaryCard(width int, title, value, detail string) string {
	innerW := innerWidthForStyle(summaryCardStyle, width)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		renderBoundedText(summaryTitleStyle, title, innerW),
		renderBoundedText(summaryValueStyle, value, innerW),
		renderBoundedText(summarySubtleStyle, detail, innerW),
	)
	return summaryCardStyle.Width(innerW).Render(content)
}

func (m Model) renderFilterLine() string {
	filterLabel := "/ all conversations"
	if m.filtering {
		filterLabel = "/ " + m.filterText + "█"
	} else if m.filterText != "" {
		filterLabel = "/ " + m.filterText
	}
	return renderBoundedText(filterStyle, filterLabel+"  sort "+sortLabels[m.sortBy], max(1, m.leftPanelInnerWidth()-1))
}

func (m Model) selectionFooterLine() string {
	cursor := 0
	if len(m.filtered) > 0 {
		cursor = m.cursor + 1
	}
	return renderBoundedText(tableMetaStyle, fmt.Sprintf("%d visible  row %d/%d", len(m.filtered), cursor, len(m.filtered)), max(1, m.leftPanelInnerWidth()-1))
}

func (m Model) modeSummary() string {
	switch m.phase {
	case phaseLoading:
		return "Loading"
	case phaseSelect:
		return "Selection"
	case phaseAction:
		return "Action Picker"
	case phaseConfirm:
		return "Confirm"
	case phaseRunning:
		return "Running"
	case phaseDone:
		return "Completed"
	case phaseError:
		return "Error"
	default:
		return "Idle"
	}
}

func (m Model) modeSummaryDetail() string {
	switch m.phase {
	case phaseLoading:
		return "Auth + sync"
	case phaseSelect:
		return "Mark rows"
	case phaseAction:
		return "Choose action"
	case phaseConfirm:
		return "Confirm action"
	case phaseRunning:
		return "Applying action"
	case phaseDone:
		return "Finished"
	case phaseError:
		return "See the status log"
	default:
		return "Waiting for input"
	}
}

func (m Model) cacheSummary() string {
	switch {
	case len(m.conversations) == 0 && m.phase == phaseLoading:
		return "Waiting"
	case len(m.conversations) == 0:
		return "Empty"
	case m.loadingText != "":
		return "Warm"
	default:
		return "Ready"
	}
}

func (m Model) cacheSummaryDetail() string {
	switch {
	case len(m.conversations) == 0 && m.phase == phaseLoading:
		return "No cache rendered yet"
	case len(m.conversations) == 0:
		return "No visible cached rows"
	default:
		return fmt.Sprintf("%d cached locally", len(m.conversations))
	}
}

func (m Model) selectionSummary() string {
	if len(m.selected) == 0 {
		return "0 selected"
	}
	return fmt.Sprintf("%d selected", len(m.selected))
}

func (m Model) selectionSummaryDetail() string {
	if len(m.filtered) == 0 {
		return "No visible rows"
	}
	return fmt.Sprintf("row %d/%d", m.cursor+1, len(m.filtered))
}

func (m Model) nextActionSummary() (string, string) {
	switch m.phase {
	case phaseAction:
		return actionLabels[m.actionCursor], "Press enter to keep the highlighted action or esc to go back."
	case phaseConfirm:
		return "Confirm " + strings.ToLower(actionLabels[m.actionCursor]), "Press y to continue or esc to return to action selection."
	case phaseRunning:
		return "Processing", "The current behavior is running without UI changes to the action semantics."
	case phaseDone:
		return "Review results", "Press q to leave the terminal UI."
	case phaseError:
		return "Exit", "Press enter or q to leave the terminal UI."
	default:
		if len(m.selected) == 0 {
			return "Choose rows", "Use j/k to move, space to toggle, then press enter to open actions."
		}
		return "Open actions", "Press enter to choose archive, delete, or cancel for the current selection."
	}
}

func (m Model) workspaceColumnWidths() (int, int) {
	total := m.contentWidth()
	available := max(1, total-1)
	left := available * 68 / 100
	right := available - left
	if left < 48 && available >= 48+24 {
		left = 48
		right = available - left
	}
	if right < 24 && available >= 48+24 {
		right = 24
		left = available - right
	}
	return left, right
}

func (m Model) leftPanelWidth() int {
	left, _ := m.workspaceColumnWidths()
	return left
}

func (m Model) rightPanelWidth() int {
	_, right := m.workspaceColumnWidths()
	return right
}

func (m Model) leftPanelInnerWidth() int {
	return max(1, m.leftPanelWidth()-4)
}

func (m Model) rightPanelInnerWidth() int {
	return max(1, m.rightPanelWidth()-4)
}

func (m Model) conversationColumnWidths(contentWidth int) []int {
	checkWidth := 4
	updatedWidth := 11
	stateWidth := 8
	titleWidth := max(18, contentWidth-checkWidth-updatedWidth-stateWidth-4)
	return []int{checkWidth, titleWidth, updatedWidth, stateWidth}
}

func (m Model) sidebarPanelHeights(total int) (int, int, int) {
	available := max(3, total-2)
	top := 8
	middle := 8
	if available < top+middle+6 {
		top = max(5, available/3)
		middle = max(5, available/4)
	}
	bottom := available - top - middle
	if bottom < 6 {
		short := 6 - bottom
		if middle-short >= 5 {
			middle -= short
		} else if top-short >= 5 {
			top -= short
		}
		bottom = available - top - middle
	}
	return top, middle, max(6, bottom)
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

func padRightPlain(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = trim(value, width)
	padding := max(0, width-lipgloss.Width(value))
	return value + strings.Repeat(" ", padding)
}

func padLeftPlain(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = trim(value, width)
	padding := max(0, width-lipgloss.Width(value))
	return strings.Repeat(" ", padding) + value
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
	// Wrap first, then take the tail, so a single long debug line does not
	// consume the entire viewport width or hide the latest events.
	wrapped := make([]string, 0, len(m.logs))
	for _, line := range m.logs {
		wrapped = append(wrapped, wrapLines(line, width)...)
	}
	if len(wrapped) <= 8 {
		return wrapped
	}
	return wrapped[len(wrapped)-8:]
}

func (m Model) visibleRange(tableHeight int) struct{ start, end int } {
	maxItems := max(8, tableHeight-1)
	if len(m.filtered) <= maxItems {
		return struct{ start, end int }{0, len(m.filtered)}
	}

	// Keep the cursor roughly centered until we hit the list boundaries.
	start := m.cursor - maxItems/2
	if start < 0 {
		start = 0
	}
	end := start + maxItems
	if end > len(m.filtered) {
		end = len(m.filtered)
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
			return loadResultMsg{err: err, refresh: true}
		}

		sortConversations(conversations)

		return loadResultMsg{conversations: conversations, refresh: true}
	}
}

func loadCachedConversationsCmd(client *chatgpt.Client) tea.Cmd {
	return func() tea.Msg {
		conversations, err := client.CachedConversations()
		if err != nil {
			return loadResultMsg{err: err}
		}

		sortConversations(conversations)

		return loadResultMsg{conversations: conversations}
	}
}

func sortConversations(conversations []chatgpt.Conversation) {
	// Mirror the web UI by showing the most recently updated conversations first.
	slices.SortStableFunc(conversations, func(a, b chatgpt.Conversation) int {
		if a.UpdateTime.Equal(b.UpdateTime.Time) {
			return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		}
		if a.UpdateTime.After(b.UpdateTime.Time) {
			return -1
		}
		return 1
	})
}

func pollSnapshotCmd(client *chatgpt.Client) tea.Cmd {
	// Polling keeps the TUI reactive while auth/fetch work stays inside the client.
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
		// Apply mutations serially so status and rate-limiting behavior remain predictable.
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

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return 80
	}
	return max(72, m.width-4)
}

func (m Model) panelInnerWidth() int {
	innerW := m.contentWidth() - 4 // left/right border + left/right padding
	if innerW < 1 {
		return 1
	}
	return innerW
}

func (m Model) renderPanel(title, body string) string {
	return m.renderPanelWithWidth(title, body, 0, m.contentWidth())
}

func (m Model) renderPanelSized(title, body string, height int) string {
	return m.renderPanelWithWidth(title, body, height, m.contentWidth())
}

func (m Model) renderPanelWithWidth(title, body string, height int, width int) string {
	innerW := innerWidthForStyle(panelStyle, width)
	titleLine := panelTitleStyle.Width(innerW).Background(appBg).Render(title)
	clampedBody := body

	if height > 0 {
		bodyH := max(1, height-panelBodyFrameHeight())
		clampedBody = lipgloss.NewStyle().
			Width(innerW).
			Height(bodyH).
			MaxHeight(bodyH).
			Background(appBg).
			Foreground(lipgloss.Color("#D8D6EA")).
			Render(body)
	}

	return panelStyle.Width(innerW).Render(
		lipgloss.JoinVertical(lipgloss.Left, titleLine, clampedBody),
	)
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

func formatConversationRow(widths []int, values ...string) string {
	parts := make([]string, 0, len(values))
	for i, value := range values {
		if i >= 2 {
			parts = append(parts, padLeftPlain(value, widths[i]))
			continue
		}
		parts = append(parts, padRightPlain(value, widths[i]))
	}
	return strings.Join(parts, " ")
}

func renderAlignedStyledTextLine(width int, left, right string, leftStyle, rightStyle lipgloss.Style) string {
	rightLimit := max(8, min(width/3, width-2))
	rightText := trim(right, rightLimit)
	rightW := lipgloss.Width(rightText)
	leftW := max(1, width-rightW-1)
	leftText := trim(left, leftW)
	gap := max(1, width-lipgloss.Width(leftText)-rightW)
	return leftStyle.Render(leftText) + strings.Repeat(" ", gap) + rightStyle.Render(rightText)
}

func renderBoundedText(style lipgloss.Style, text string, width int) string {
	return style.Render(padRightPlain(text, width))
}

func splitFixedWidth(total, parts, gap int) []int {
	if parts <= 0 {
		return nil
	}
	available := max(parts, total-gap*(parts-1))
	base := available / parts
	remainder := available % parts
	widths := make([]int, parts)
	for i := 0; i < parts; i++ {
		widths[i] = base
		if i < remainder {
			widths[i]++
		}
	}
	return widths
}

func innerWidthForStyle(style lipgloss.Style, outer int) int {
	return max(1, outer-style.GetHorizontalFrameSize())
}

func outerHeightForStyle(style lipgloss.Style, contentRows int) int {
	return max(1, contentRows+style.GetVerticalFrameSize())
}

func panelBodyFrameHeight() int {
	return panelStyle.GetVerticalFrameSize() + 1
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

// Bubble Tea v2 key names vary a bit across terminals, so the helpers below
// match both semantic key codes and common string forms.
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

func isPageUpKey(msg tea.KeyPressMsg) bool {
	return msg.Code == tea.KeyPgUp || msg.String() == "pgup"
}

func isPageDownKey(msg tea.KeyPressMsg) bool {
	return msg.Code == tea.KeyPgDown || msg.String() == "pgdown"
}

func matchesRune(msg tea.KeyPressMsg, want string) bool {
	return strings.EqualFold(msg.String(), want)
}

// pageSize returns the number of items to skip on PgUp/PgDn using the same
// layout calculation as the rendered selection pane.
func (m Model) pageSize() int {
	h := max(24, m.height)
	headerH := lipgloss.Height(m.renderChrome())
	bodyH := h - headerH - 2
	if bodyH < 0 {
		bodyH = 0
	}
	return max(1, m.selectionTableHeight(bodyH)-1) // -1 for the table header row
}

func (m Model) selectionTableHeight(bodyH int) int {
	// Workspace panel overhead: 2 borders + 1 panel title + 2 fixed body rows
	// (filter and footer).
	return max(8, bodyH-5)
}
