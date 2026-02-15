package tui

import (
	"context"
	"fmt"
	"os/user"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zhengda-lu/whport/internal/port"
	"github.com/zhengda-lu/whport/internal/process"
)

// viewState tracks which screen the TUI is currently showing.
type viewState int

const (
	viewTable viewState = iota
	viewInfo
	viewKillConfirm
	viewKillResult
	viewFilter
)

// sortField defines what column to sort by.
type sortField int

const (
	sortByPort sortField = iota
	sortByPID
	sortByProcess
)

// Messages for async operations.
type scanDoneMsg struct {
	entries []port.PortEntry
	err     error
}

type tickMsg time.Time

type killDoneMsg struct {
	pid     int
	process string
	port    int
	err     error
	forced  bool
}

type infoDoneMsg struct {
	info *process.ProcessInfo
	err  error
}

// Model is the main Bubbletea model for the whport TUI.
type Model struct {
	scanner  *port.LsofScanner
	manager  *process.RealManager
	version  string
	entries  []port.PortEntry
	filtered []int // indices into entries for currently displayed items

	cursor       int
	scrollOffset int
	sortBy       sortField
	searching    bool
	searchQuery  string
	paused       bool

	// Info view state.
	infoEntry *port.PortEntry
	infoData  *process.ProcessInfo
	infoErr   error

	// Kill confirmation state.
	killEntry  *port.PortEntry
	killResult string
	killErr    error

	currentUser string
	scanning    bool
	spinner     spinner.Model

	width  int
	height int

	currentView viewState
}

// New creates a new TUI model.
func New(scanner *port.LsofScanner, manager *process.RealManager, version string) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorCyan)

	currentUser := "unknown"
	if u, err := user.Current(); err == nil {
		currentUser = u.Username
	}

	return Model{
		scanner:     scanner,
		manager:     manager,
		version:     version,
		currentUser: currentUser,
		scanning:    true,
		spinner:     sp,
		currentView: viewTable,
	}
}

// Init starts the spinner and kicks off the initial scan.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.doScan(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) doScan() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		entries, err := m.scanner.ListPorts(ctx)
		return scanDoneMsg{entries: entries, err: err}
	}
}

func (m Model) doScanAll() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		entries, err := m.scanner.ListAllPorts(ctx)
		return scanDoneMsg{entries: entries, err: err}
	}
}

func (m Model) doKill(pid int, processName string, portNum int, force bool) tea.Cmd {
	mgr := m.manager
	return func() tea.Msg {
		if force {
			err := mgr.ForceKill(pid)
			return killDoneMsg{pid: pid, process: processName, port: portNum, err: err, forced: true}
		}
		exited, err := mgr.GracefulKill(pid)
		if err != nil {
			return killDoneMsg{pid: pid, process: processName, port: portNum, err: err}
		}
		if !exited {
			return killDoneMsg{
				pid:     pid,
				process: processName,
				port:    portNum,
				err:     fmt.Errorf("process did not exit after SIGTERM (still running)"),
			}
		}
		return killDoneMsg{pid: pid, process: processName, port: portNum}
	}
}

func (m Model) doGetInfo(pid int) tea.Cmd {
	mgr := m.manager
	return func() tea.Msg {
		ctx := context.Background()
		info, err := mgr.Info(ctx, pid)
		return infoDoneMsg{info: info, err: err}
	}
}

// Update handles all messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		if m.scanning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tickMsg:
		if !m.paused && m.currentView == viewTable {
			return m, tea.Batch(m.doScan(), tickCmd())
		}
		return m, tickCmd()

	case scanDoneMsg:
		m.scanning = false
		if msg.err == nil {
			m.entries = msg.entries
			m.sortEntries()
			m.rebuildFiltered()
		}
		return m, nil

	case killDoneMsg:
		m.killErr = msg.err
		if msg.err == nil {
			m.killResult = fmt.Sprintf("Killed %s (PID %d) on port %d", msg.process, msg.pid, msg.port)
			if msg.forced {
				m.killResult = fmt.Sprintf("Force killed %s (PID %d) on port %d", msg.process, msg.pid, msg.port)
			}
		}
		m.currentView = viewKillResult
		return m, nil

	case infoDoneMsg:
		m.infoData = msg.info
		m.infoErr = msg.err
		m.currentView = viewInfo
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.currentView {
		case viewTable:
			return m.updateTable(msg)
		case viewInfo:
			return m.updateInfo(msg)
		case viewKillConfirm:
			return m.updateKillConfirm(msg)
		case viewKillResult:
			return m.updateKillResult(msg)
		case viewFilter:
			return m.updateFilter(msg)
		}
	}

	return m, nil
}

func (m Model) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "j", "down":
		if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.ensureCursorVisible()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
		}
	case "K":
		if entry := m.selectedEntry(); entry != nil {
			m.killEntry = entry
			m.currentView = viewKillConfirm
		}
	case "i", "enter":
		if entry := m.selectedEntry(); entry != nil {
			m.infoEntry = entry
			m.infoData = nil
			m.infoErr = nil
			return m, m.doGetInfo(entry.PID)
		}
	case "r":
		m.scanning = true
		return m, tea.Batch(m.doScan(), m.spinner.Tick)
	case "s":
		m.sortBy = (m.sortBy + 1) % 3
		m.sortEntries()
		m.rebuildFiltered()
	case "p":
		m.paused = !m.paused
	case "/":
		m.currentView = viewFilter
		m.searchQuery = ""
		m.searching = true
	case "esc":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.searching = false
			m.rebuildFiltered()
		}
	}
	return m, nil
}

func (m Model) updateInfo(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc", "backspace":
		m.currentView = viewTable
	case "K":
		if m.infoEntry != nil {
			m.killEntry = m.infoEntry
			m.currentView = viewKillConfirm
		}
	}
	return m, nil
}

func (m Model) updateKillConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		if m.killEntry != nil {
			e := m.killEntry
			return m, m.doKill(e.PID, e.Process, e.Port, false)
		}
	case "f":
		if m.killEntry != nil {
			e := m.killEntry
			return m, m.doKill(e.PID, e.Process, e.Port, true)
		}
	case "n", "esc", "N":
		m.currentView = viewTable
		m.killEntry = nil
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateKillResult(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc", "enter", "backspace":
		m.currentView = viewTable
		m.killEntry = nil
		m.killResult = ""
		m.killErr = nil
		// Refresh after kill.
		m.scanning = true
		return m, tea.Batch(m.doScan(), m.spinner.Tick)
	}
	return m, nil
}

func (m Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.currentView = viewTable
		m.searching = false
		m.rebuildFiltered()
	case "esc":
		m.currentView = viewTable
		m.searching = false
		m.searchQuery = ""
		m.rebuildFiltered()
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.rebuildFiltered()
		}
	default:
		key := msg.String()
		if len(key) == 1 {
			m.searchQuery += key
			m.rebuildFiltered()
		}
	}
	return m, nil
}

func (m *Model) selectedEntry() *port.PortEntry {
	if len(m.filtered) == 0 || m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	idx := m.filtered[m.cursor]
	if idx >= len(m.entries) {
		return nil
	}
	entry := m.entries[idx]
	return &entry
}

func (m *Model) sortEntries() {
	sort.SliceStable(m.entries, func(i, j int) bool {
		switch m.sortBy {
		case sortByPID:
			return m.entries[i].PID < m.entries[j].PID
		case sortByProcess:
			return strings.ToLower(m.entries[i].Process) < strings.ToLower(m.entries[j].Process)
		default:
			return m.entries[i].Port < m.entries[j].Port
		}
	})
}

func (m *Model) rebuildFiltered() {
	m.filtered = m.filtered[:0]
	query := strings.ToLower(m.searchQuery)
	for i, e := range m.entries {
		if query != "" {
			match := strings.Contains(strings.ToLower(e.Process), query) ||
				strings.Contains(strings.ToLower(e.User), query) ||
				strings.Contains(strings.ToLower(e.Command), query) ||
				strings.Contains(fmt.Sprintf("%d", e.Port), query) ||
				strings.Contains(fmt.Sprintf("%d", e.PID), query)
			if !match {
				continue
			}
		}
		m.filtered = append(m.filtered, i)
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	m.adjustScroll()
}

func (m *Model) ensureCursorVisible() {
	visible := m.visibleRows()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
}

func (m *Model) adjustScroll() {
	visible := m.visibleRows()
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
	maxOffset := len(m.filtered) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m Model) visibleRows() int {
	// Reserve lines for: header (2), column headers (1), separator (1), status bar (2), help (1) = 7.
	const reserved = 7
	visible := m.height - reserved
	if visible < 1 {
		visible = 1
	}
	return visible
}

// View renders the TUI.
func (m Model) View() string {
	switch m.currentView {
	case viewInfo:
		return m.viewInfo()
	case viewKillConfirm:
		return m.viewKillConfirm()
	case viewKillResult:
		return m.viewKillResult()
	case viewFilter:
		return m.viewFilter()
	default:
		return m.viewTable()
	}
}

func (m Model) viewTable() string {
	var b strings.Builder

	// Header bar.
	title := titleStyle.Render(fmt.Sprintf("whport %s", m.version))
	listenCount := 0
	for _, e := range m.entries {
		if e.State == "LISTEN" {
			listenCount++
		}
	}
	stats := dimStyle.Render(fmt.Sprintf("Listening: %d  Total: %d", listenCount, len(m.entries)))
	pauseIndicator := ""
	if m.paused {
		pauseIndicator = warnStyle.Render("  [PAUSED]")
	}
	b.WriteString(title + "  " + stats + pauseIndicator + "\n")

	if m.scanning && len(m.entries) == 0 {
		b.WriteString("\n" + m.spinner.View() + " Scanning ports...\n")
		return b.String()
	}

	// Column headers.
	sortIndicator := func(field sortField) string {
		if m.sortBy == field {
			return " ^"
		}
		return ""
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf(
		"  %-7s %-6s %-7s %-16s %-11s %-13s %s",
		"PORT"+sortIndicator(sortByPort),
		"PROTO",
		"PID"+sortIndicator(sortByPID),
		"PROCESS"+sortIndicator(sortByProcess),
		"USER",
		"STATE",
		"COMMAND",
	)) + "\n")

	if len(m.filtered) == 0 {
		if m.searchQuery != "" {
			b.WriteString("\n  No results matching: " + m.searchQuery + "\n")
		} else {
			b.WriteString("\n  No listening ports found.\n")
		}
	} else {
		viewportRows := m.visibleRows()
		end := m.scrollOffset + viewportRows
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := m.scrollOffset; i < end; i++ {
			idx := m.filtered[i]
			e := m.entries[idx]

			cursor := "  "
			if i == m.cursor {
				cursor = cursorStyle.Render("> ")
			}

			// Truncate command to fit.
			cmd := e.Command
			maxCmdLen := m.width - 60
			if maxCmdLen < 10 {
				maxCmdLen = 10
			}
			if len(cmd) > maxCmdLen {
				cmd = cmd[:maxCmdLen-3] + "..."
			}

			style := processStyle(e.User)
			line := fmt.Sprintf("%-7d %-6s %-7d %-16s %-11s %-13s %s",
				e.Port, e.Protocol, e.PID,
				truncate(e.Process, 16),
				truncate(e.User, 11),
				e.State,
				cmd,
			)

			b.WriteString(cursor + style.Render(line) + "\n")
		}

		// Scroll indicator.
		if len(m.filtered) > viewportRows {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  [%d-%d of %d]",
				m.scrollOffset+1, end, len(m.filtered))) + "\n")
		}
	}

	// Search indicator.
	if m.searchQuery != "" {
		b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("  filter: %s", m.searchQuery)))
	}

	// Help bar.
	b.WriteString(helpStyle.Render("j/k:navigate  K:kill  i:info  r:refresh  s:sort  p:pause  /:search  q:quit") + "\n")

	return b.String()
}

func (m Model) viewInfo() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("whport -- Port Info") + "\n\n")

	if m.infoErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", m.infoErr)) + "\n")
		b.WriteString(helpStyle.Render("\nesc back | q quit") + "\n")
		return b.String()
	}

	if m.infoEntry == nil {
		b.WriteString("  No port selected.\n")
		b.WriteString(helpStyle.Render("\nesc back | q quit") + "\n")
		return b.String()
	}

	e := m.infoEntry
	b.WriteString(labelStyle.Render("Port:") + valueStyle.Render(fmt.Sprintf("%d/%s", e.Port, e.Protocol)) + "\n")
	b.WriteString(labelStyle.Render("State:") + valueStyle.Render(e.State) + "\n")
	b.WriteString(labelStyle.Render("Process:") + valueStyle.Render(fmt.Sprintf("%s (PID %d)", e.Process, e.PID)) + "\n")

	if m.infoData != nil {
		info := m.infoData
		b.WriteString(labelStyle.Render("Command:") + valueStyle.Render(info.Command) + "\n")
		b.WriteString(labelStyle.Render("User:") + valueStyle.Render(info.User) + "\n")

		if !info.StartTime.IsZero() {
			ago := time.Since(info.StartTime).Truncate(time.Second)
			b.WriteString(labelStyle.Render("Started:") + valueStyle.Render(
				fmt.Sprintf("%s ago (%s)", formatDuration(ago), info.StartTime.Format("2006-01-02 15:04:05")),
			) + "\n")
		}

		b.WriteString(labelStyle.Render("CPU:") + valueStyle.Render(fmt.Sprintf("%.1f%%", info.CPUPercent)) + "\n")
		b.WriteString(labelStyle.Render("Memory:") + valueStyle.Render(formatBytes(info.MemRSS)+" (RSS)") + "\n")

		if info.PPID > 0 {
			b.WriteString(labelStyle.Render("Parent PID:") + valueStyle.Render(fmt.Sprintf("%d", info.PPID)) + "\n")
		}

		if len(info.Children) > 0 {
			childStrs := make([]string, len(info.Children))
			for i, c := range info.Children {
				childStrs[i] = fmt.Sprintf("%d", c)
			}
			b.WriteString(labelStyle.Render("Children:") + valueStyle.Render(strings.Join(childStrs, ", ")) + "\n")
		}
	} else {
		b.WriteString(labelStyle.Render("User:") + valueStyle.Render(e.User) + "\n")
	}

	b.WriteString(helpStyle.Render("\nK:kill  esc:back  q:quit") + "\n")
	return b.String()
}

func (m Model) viewKillConfirm() string {
	var b strings.Builder

	b.WriteString(dangerStyle.Render(" KILL PROCESS ") + "\n\n")

	if m.killEntry == nil {
		b.WriteString("  No process selected.\n")
		b.WriteString(helpStyle.Render("\nesc cancel | q quit") + "\n")
		return b.String()
	}

	e := m.killEntry
	b.WriteString(fmt.Sprintf("  Kill process %q (PID %d) on port %d?\n\n",
		e.Process, e.PID, e.Port))

	if e.User == "root" || e.User != m.currentUser {
		b.WriteString(warnStyle.Render("  WARNING: This process belongs to user '"+e.User+"'.") + "\n")
		b.WriteString(warnStyle.Render("  You may need elevated privileges to kill it.") + "\n\n")
	}

	b.WriteString("  " + dimStyle.Render("[y] SIGTERM (graceful)  [f] SIGKILL (force)  [n] cancel") + "\n")
	b.WriteString(helpStyle.Render("\ny:terminate  f:force  n/esc:cancel") + "\n")
	return b.String()
}

func (m Model) viewKillResult() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("whport -- Kill Result") + "\n\n")

	if m.killErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Failed: %v", m.killErr)) + "\n")
	} else {
		b.WriteString(successStyle.Render(fmt.Sprintf("  %s", m.killResult)) + "\n")
	}

	b.WriteString(helpStyle.Render("\nenter/esc:back  q:quit") + "\n")
	return b.String()
}

func (m Model) viewFilter() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("whport -- Search") + "\n\n")
	b.WriteString("  Type to filter: " + m.searchQuery + "_\n")
	b.WriteString(helpStyle.Render("\nenter:apply  esc:cancel") + "\n")

	return b.String()
}

// truncate truncates a string to max length, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatBytes formats bytes into a human-readable string.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// formatDuration formats a duration into a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, int(d.Minutes())%60)
	}
	days := hours / 24
	return fmt.Sprintf("%dd %dh", days, hours%24)
}

// signalName returns the human-readable name for a signal.
func signalName(sig syscall.Signal) string {
	switch sig {
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGKILL:
		return "SIGKILL"
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGHUP:
		return "SIGHUP"
	default:
		return fmt.Sprintf("signal(%d)", sig)
	}
}
