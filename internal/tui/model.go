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

type sortMode int

const (
	sortDateDesc sortMode = iota
	sortDateAsc
	sortTitleAsc
	sortTitleDesc
)

var sortLabels = []string{"Date ↓", "Date ↑", "Title A-Z", "Title Z-A"}


var (
	baseAppStyle       = lipgloss.NewStyle().PaddingTop(1).PaddingLeft(1).PaddingRight(1).PaddingBottom(0).Background(lipgloss.Color("#1A1826")).Foreground(lipgloss.Color("#D8D6EA"))
	appTitleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4B942"))
	phaseBadgeStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#1A1826")).Background(lipgloss.Color("#5FD7FF")).Padding(0, 1)
	metaKeyStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4B942"))
	metaValueStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8D6EA"))
	headerBoxStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5FD7FF")).Padding(0, 1)
	headerSepStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#5A5878"))
	headerDividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5A5878")).Border(lipgloss.Border{Bottom: "─"}).BorderForeground(lipgloss.Color("#5A5878"))
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FD7FF"))
	subtleStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A88A8"))
	selectedLineStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7FFFD4")).Bold(true)
	helpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8A3C2"))
	warningStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#F7C95C"))
	errorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Bold(true)
	successStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#6AE3A8")).Bold(true)
	statusPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5FD7FF")).Padding(1, 2)
	errorBoxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#FF7B72")).Padding(1, 2)
	statusBannerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6AE3A8")).Bold(true)
	logViewportStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D8D6EA"))
	panelStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#5FD7FF")).Padding(0, 1)
	panelTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5FD7FF"))
	tableMetaStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A88A8"))
	tableBoxStyle      = lipgloss.NewStyle()
	tableHeaderStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F3F0FF")).Underline(true)
	tableRowStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BD5FF"))
	selectedRowStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#1A1826")).Background(lipgloss.Color("#C084FC")).Bold(true)
	filterStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A88A8")).PaddingLeft(1)
	filterActiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4B942")).Bold(true).PaddingLeft(1)
	shortcutRowStyle   = lipgloss.NewStyle()
	shortcutKeyStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F4B942"))
	shortcutDescStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A88A8"))
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
	return tea.Batch(loadConversationsCmd(m.client), pollSnapshotCmd(m.client))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case loadResultMsg:
		// Conversation loading is kicked off immediately in Init and moves the
		// model from auth/loading into the selectable list view.
		if msg.err != nil {
			m.phase = phaseError
			m.err = msg.err
			return m, nil
		}
		m.conversations = msg.conversations
		m.applyFilterAndSort()
		m.phase = phaseSelect
		if len(m.conversations) == 0 {
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
	w := max(80, m.width)
	h := max(24, m.height)

	// 1. Render header (fixed)
	header := m.renderChrome()

	// 3. Render body content (fills remaining space).
	//    Subtract 1 for the top padding in baseAppStyle (bottom padding is 0).
	headerH := lipgloss.Height(header)
	bodyH := h - headerH - 1
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

	v := tea.NewView(baseAppStyle.Width(w).Height(h).Render(content))
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
		return "j/k move   space mark   a all   / filter   s sort   enter actions   q quit"
	case phaseAction:
		return "j/k move   enter choose   esc back"
	case phaseConfirm:
		return "y confirm   esc back"
	case phaseRunning:
		return "q quit"
	case phaseDone:
		return "enter exit   q quit"
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
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
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
	if len(m.conversations) == 0 {
		return m.renderPanelSized("Conversations", statusPanelStyle.Width(m.contentWidth()).Render("No conversations found for this account."), bodyH)
	}

	tableHeight := max(8, bodyH-6) // leave room for meta + panel borders

	// Build meta line with filter and sort info
	metaParts := []string{
		fmt.Sprintf("%d/%d shown", len(m.filtered), len(m.conversations)),
		fmt.Sprintf("%d selected", len(m.selected)),
		fmt.Sprintf("sort: %s", sortLabels[m.sortBy]),
	}
	metaLine := tableMetaStyle.Render(strings.Join(metaParts, "   "))

	// Filter bar
	filterLine := ""
	if m.filtering {
		filterLine = filterActiveStyle.Render("/ " + m.filterText + "█")
	} else if m.filterText != "" {
		filterLine = filterStyle.Render("/ " + m.filterText)
	}

	bodyParts := []string{metaLine}
	if filterLine != "" {
		bodyParts = append(bodyParts, filterLine)
		tableHeight -= 1
	}
	bodyParts = append(bodyParts, m.renderConversationTable(tableHeight))

	panelTitle := fmt.Sprintf("Conversations [%d]", len(m.filtered))
	return m.renderPanelSized(panelTitle, lipgloss.JoinVertical(lipgloss.Left, bodyParts...), bodyH)
}

func (m Model) renderActionPicker(bodyH int) string {
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

	return m.renderPanelSized("Actions", statusPanelStyle.Width(m.contentWidth()).Render(options.String()), bodyH)
}

func (m Model) renderConfirmation(bodyH int) string {
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
	return m.renderPanelSized("Confirm", statusPanelStyle.Width(m.contentWidth()).Render(panel.String()), bodyH)
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
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		statusBannerStyle.Width(m.contentWidth()).Render(m.loadingText),
		"",
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

	// Title row: app name left, phase badge right
	title := appTitleStyle.Render("bulk-delete-chatgpt-conversations")
	badge := phaseBadgeStyle.Render(m.phaseLabel())
	gap := max(0, w-lipgloss.Width(title)-lipgloss.Width(badge)-4)
	titleRow := title + strings.Repeat(" ", gap) + badge

	// Info row: key-value pairs separated by a dim divider
	sep := headerSepStyle.Render(" │ ")
	infoItems := []string{
		metaKeyStyle.Render("Email ") + metaValueStyle.Render(valueOrPlaceholder(m.email)),
		metaKeyStyle.Render("Session ") + metaValueStyle.Render(valueOrPlaceholder(m.sessionID)),
		metaKeyStyle.Render("") + metaValueStyle.Render(valueOrPlaceholder(m.version)),
		metaKeyStyle.Render("Go ") + metaValueStyle.Render(runtime.Version()),
	}
	infoRow := strings.Join(infoItems, sep)
	hintRow := m.renderShortcutHints(w - 4)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleRow,
		headerDividerStyle.Width(w-4).Render(""),
		infoRow,
		headerDividerStyle.Width(w-4).Render(""),
		hintRow,
	)
	return headerBoxStyle.Width(w).Render(content)
}

func (m Model) renderConversationTable(height int) string {
	visible := m.visibleRange(height)
	headers := []string{"Select", "Conversation Title", "Date", "Age"}
	contentWidth := m.contentWidth() - 2
	widths := []int{10, max(24, contentWidth-38), 14, 8}

	var rows []string
	rows = append(rows, tableHeaderStyle.Render(
		formatTableRow(widths, headers[0], headers[1], headers[2], headers[3]),
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
			return loadResultMsg{err: err}
		}

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

		return loadResultMsg{conversations: conversations}
	}
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
		return 100
	}
	return max(72, m.width-4)
}

func (m Model) renderPanel(title, body string) string {
	return panelStyle.Width(m.contentWidth()).Render(
		lipgloss.JoinVertical(lipgloss.Left, panelTitleStyle.Render(title), body),
	)
}

func (m Model) renderPanelSized(title, body string, height int) string {
	if height <= 0 {
		return m.renderPanel(title, body)
	}
	// panelStyle has a RoundedBorder (1 line top + 1 line bottom) so the
	// content height must be reduced by 2 to keep the total rendered
	// height equal to the requested height.
	contentH := height - 2
	if contentH < 1 {
		contentH = 1
	}
	return panelStyle.Width(m.contentWidth()).Height(contentH).Render(
		lipgloss.JoinVertical(lipgloss.Left, panelTitleStyle.Render(title), body),
	)
}

func (m Model) renderShortcutHints(width int) string {
	items := splitFooterItems(m.shortcutHints())
	var formatted []string
	for _, item := range items {
		parts := strings.SplitN(item, " ", 2)
		if len(parts) == 2 {
			formatted = append(formatted, shortcutKeyStyle.Render("<"+parts[0]+">")+shortcutDescStyle.Render(parts[1]))
		} else {
			formatted = append(formatted, shortcutKeyStyle.Render("<"+item+">"))
		}
	}
	content := strings.Join(formatted, "  ")
	if lipgloss.Width(content) > width {
		mid := len(formatted) / 2
		line1 := strings.Join(formatted[:mid], "  ")
		line2 := strings.Join(formatted[mid:], "  ")
		content = line1 + "\n" + line2
	}
	return shortcutRowStyle.Width(width).Render(content)
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

func splitFooterItems(text string) []string {
	parts := strings.Split(text, "   ")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
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

func matchesRune(msg tea.KeyPressMsg, want string) bool {
	return strings.EqualFold(msg.String(), want)
}
