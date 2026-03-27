package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/trekmax/cloudguard-cli/internal/client"
	"github.com/trekmax/cloudguard-cli/internal/output"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginBottom(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			MarginRight(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	valueStyle = lipgloss.NewStyle().
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	greenBar  = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	yellowBar = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	redBar    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// Model is the bubbletea model for the dashboard.
type Model struct {
	client   *client.Client
	interval time.Duration
	status   *output.StatusDisplay
	sysInfo  *client.SystemInfo
	err      error
	width    int
	height   int
	quitting bool
}

type tickMsg time.Time
type statusMsg struct {
	status  *output.StatusDisplay
	sysInfo *client.SystemInfo
	err     error
}

// Run starts the TUI dashboard.
func Run(c *client.Client, intervalSec int) error {
	m := Model{
		client:   c,
		interval: time.Duration(intervalSec) * time.Second,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchStatus(m.client), tickCmd(m.interval))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, tea.Batch(fetchStatus(m.client), tickCmd(m.interval))

	case statusMsg:
		m.err = msg.err
		if msg.status != nil {
			m.status = msg.status
		}
		if msg.sysInfo != nil {
			m.sysInfo = msg.sysInfo
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("CloudGuard Monitor Dashboard"))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
	}

	if m.status == nil {
		b.WriteString(labelStyle.Render("Connecting..."))
		b.WriteString("\n")
		return b.String()
	}

	s := m.status

	// Row 1: CPU + Memory
	cpuBox := renderCPUBox(s)
	memBox := renderMemoryBox(s)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cpuBox, memBox))
	b.WriteString("\n")

	// Row 2: Network + System Info
	netBox := renderNetworkBox(s)
	sysBox := renderSystemBox(m.sysInfo)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, netBox, sysBox))
	b.WriteString("\n")

	// Footer
	b.WriteString(labelStyle.Render(fmt.Sprintf("Auto-refresh: %s | Press 'q' to quit", m.interval)))

	return b.String()
}

func fetchStatus(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		raw, err := c.GetStatus()
		if err != nil {
			return statusMsg{err: err}
		}

		status := output.FromStatusData(raw.CPU, raw.Memory, raw.Network)

		sysInfo, _ := c.GetSystem()

		return statusMsg{status: status, sysInfo: sysInfo}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func renderCPUBox(s *output.StatusDisplay) string {
	bar := renderProgressBar(s.CPU.Usage, 30)
	content := fmt.Sprintf(
		"%s %s\n%s %.2f / %.2f / %.2f\n%s %.0f cores",
		labelStyle.Render("CPU"),
		bar,
		labelStyle.Render("Load"),
		s.CPU.Load1, s.CPU.Load5, s.CPU.Load15,
		labelStyle.Render("Cores"),
		s.CPU.Cores,
	)
	return boxStyle.Width(38).Render(content)
}

func renderMemoryBox(s *output.StatusDisplay) string {
	bar := renderProgressBar(s.Memory.UsagePercent, 30)
	content := fmt.Sprintf(
		"%s %s\n%s %s / %s\n%s %.1f%% (%s / %s)",
		labelStyle.Render("MEM"),
		bar,
		labelStyle.Render("Used"),
		fmtBytes(s.Memory.Used), fmtBytes(s.Memory.Total),
		labelStyle.Render("Swap"),
		s.Memory.SwapPercent,
		fmtBytes(s.Memory.SwapUsed), fmtBytes(s.Memory.SwapTotal),
	)
	return boxStyle.Width(38).Render(content)
}

func renderNetworkBox(s *output.StatusDisplay) string {
	content := fmt.Sprintf(
		"%s %s/s    %s %s/s\n%s %.0f",
		labelStyle.Render("RX"),
		fmtBytes(s.Network.RxRate),
		labelStyle.Render("TX"),
		fmtBytes(s.Network.TxRate),
		labelStyle.Render("Connections"),
		s.Network.Connections,
	)
	return boxStyle.Width(38).Render(content)
}

func renderSystemBox(info *client.SystemInfo) string {
	if info == nil {
		return boxStyle.Width(38).Render(labelStyle.Render("Loading system info..."))
	}

	uptime := formatUptimeShort(info.Uptime)
	content := fmt.Sprintf(
		"%s %s\n%s %s\n%s %s",
		labelStyle.Render("Host"),
		valueStyle.Render(info.Hostname),
		labelStyle.Render("OS"),
		info.OS,
		labelStyle.Render("Uptime"),
		uptime,
	)
	return boxStyle.Width(38).Render(content)
}

func renderProgressBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	filled := int(pct / 100 * float64(width))
	empty := width - filled

	var style lipgloss.Style
	switch {
	case pct >= 90:
		style = redBar
	case pct >= 70:
		style = yellowBar
	default:
		style = greenBar
	}

	bar := style.Render(strings.Repeat("█", filled)) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s] %5.1f%%", bar, pct)
}

func fmtBytes(b float64) string {
	const (
		KB = 1024.0
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1fG", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.1fM", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.1fK", b/KB)
	default:
		return fmt.Sprintf("%.0fB", b)
	}
}

func formatUptimeShort(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	mins := (seconds % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}
