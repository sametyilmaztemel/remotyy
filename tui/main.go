// Package main implements a Bubble Tea TUI client for remotty.
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog"
	"github.com/sametyilmaztemel/remotty/internal/client"
	"github.com/sametyilmaztemel/remotty/internal/config"
)

var (
	appStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Background(lipgloss.Color("#0a0a0a"))

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a3a3a3")).
			Background(lipgloss.Color("#111111")).
			Padding(0, 1).
			Bold(true).
			Width(80)

	hostStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e0e0e0")).
			Background(lipgloss.Color("#141414")).
			Padding(0, 1).
			Width(80)

	selectedStyle = hostStyle.Copy().
			Foreground(lipgloss.Color("#8b5cf6")).
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#8b5cf6"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true).
			Width(80)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22c55e")).
			Width(80)
)

type model struct {
	state    string // "init", "hosts", "connecting", "terminal"
	hosts    []string
	cursor   int
	status   string
	signalURL string
	cfg      config.ClientConfig
}

func initialModel() model {
	return model{
		state:     "init",
		signalURL: "ws://localhost:9000",
		cfg: config.ClientConfig{
			SignalURL: "ws://localhost:9000",
		},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		listHosts(m.signalURL),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.hosts)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.hosts) > 0 {
				m.state = "connecting"
				m.status = fmt.Sprintf("Connecting to %s...", m.hosts[m.cursor])
				return m, connectToHost(m.signalURL, m.hosts[m.cursor])
			}
		case "r":
			m.status = "Refreshing..."
			return m, listHosts(m.signalURL)
		}

	case hostsMsg:
		m.hosts = msg
		m.state = "hosts"
		m.status = fmt.Sprintf("%d host(s) available — press Enter to connect, r to refresh, q to quit", len(m.hosts))

	case statusMsg:
		m.status = string(msg)

	case errorMsg:
		m.status = fmt.Sprintf("Error: %s", string(msg))
	}

	return m, nil
}

func (m model) View() string {
	s := titleStyle.Render(" ⎈ remotty — TUI Client ") + "\n\n"

	if m.state == "init" {
		s += statusStyle.Render(" Connecting...")
		return appStyle.Render(s)
	}

	s += statusStyle.Render(m.status) + "\n\n"

	for i, host := range m.hosts {
		cursor := "  "
		style := hostStyle
		if i == m.cursor {
			cursor = "▸ "
			style = selectedStyle
		}
		s += style.Render(cursor + host) + "\n"
	}

	s += "\n" + infoStyle.Render("↑/↓ navigate · Enter connect · r refresh · q quit")
	return appStyle.Render(s)
}

// Messages
type hostsMsg []string
type statusMsg string
type errorMsg string

func listHosts(signalURL string) tea.Cmd {
	return func() tea.Msg {
		c, err := client.NewClient(config.ClientConfig{
			SignalURL: signalURL,
		}, zerolog.Nop())
		if err != nil {
			return errorMsg(err.Error())
		}

		hosts, err := c.ListHosts()
		if err != nil {
			return errorMsg(err.Error())
		}

		names := make([]string, len(hosts))
		for i, h := range hosts {
			names[i] = fmt.Sprintf("%-20s %s/%-8s %s",
				h.Name, h.Platform, h.Arch, joinStrings(h.Features, ", "))
		}
		return hostsMsg(names)
	}
}

func connectToHost(signalURL, hostName string) tea.Cmd {
	return func() tea.Msg {
		// Extract host ID from display string
		parts := splitHostName(hostName)
		if parts == "" {
			return errorMsg("invalid host selection")
		}

		cfg := config.ClientConfig{
			SignalURL: signalURL,
			HostID:    parts,
		}

		c, err := client.NewClient(cfg, zerolog.Nop())
		if err != nil {
			return errorMsg(err.Error())
		}

		// Use a simple context for now
		if err := c.ConnectInteractive(nil); err != nil {
			return errorMsg(err.Error())
		}
		return statusMsg("disconnected")
	}
}

func joinStrings(s []string, sep string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += sep
		}
		result += v
	}
	return result
}

func splitHostName(s string) string {
	for i, c := range s {
		if c == ' ' {
			return s[:i]
		}
	}
	return s
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
