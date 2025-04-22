package cmd

import (
	"context"
	"fmt"

	"github.com/deif/iectl/mdns"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/miekg/dns"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var updates chan []*mdns.Target

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "browse DEIF devices on the network",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		asJson, _ := cmd.Flags().GetBool("json")
		if asJson {
			return fmt.Errorf("browse cant do --json")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn("_base-unit-deif._tcp.local"), dns.TypePTR)

		browser := mdns.Browser{Question: *msg}

		var err error
		updates, err = browser.Run(context.Background())
		if err != nil {
			return fmt.Errorf("unable to browse mdns: %w", err)
		}

		items := []list.Item{}
		m := model{
			spinner: spinner.New(spinner.WithSpinner(spinner.Meter)),
			list:    list.New(items, list.NewDefaultDelegate(), 0, 0),
		}

		m.list.Title = "Devices"
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(browseCmd)
}

func mdnsUpdates() tea.Cmd {
	return func() tea.Msg {
		return <-updates
	}
}

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
)

type item interface {
	Title() string
	Description() string
	FilterValue() string
}

type model struct {
	list    list.Model
	spinner spinner.Model
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		mdnsUpdates(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "enter" {
			s := m.list.SelectedItem()
			t, ok := s.(*mdns.Target)
			if !ok {
				panic("i dont know how to handle this")
			}

			browser.OpenURL(t.Description())
			return m, tea.Quit
		}
	case []*mdns.Target:
		i := make([]list.Item, 0)
		for _, v := range msg {
			i = append(i, v)
		}
		m.list.SetItems(i)
		return m, mdnsUpdates()
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	m.list.Title = fmt.Sprintf("Looking for devices %s", m.spinner.View())
	return docStyle.Render(m.list.View())
}
