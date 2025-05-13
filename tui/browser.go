package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/deif/iectl/mdns"
)

func BrowserModel(u chan []*mdns.Target) *model {
	m := model{
		spinner: spinner.New(spinner.WithSpinner(spinner.Meter)),
		list:    list.New(make([]list.Item, 0), list.NewDefaultDelegate(), 0, 0),
		updates: u,
	}

	m.list.Title = "Devices"

	return &m
}

func (m *model) mdnsUpdates() tea.Cmd {
	return func() tea.Msg {
		return <-m.updates
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

	updates chan []*mdns.Target

	Selected []*mdns.Target
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.mdnsUpdates(),
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "enter" {
			selected := make([]*mdns.Target, 0)
			for _, v := range m.list.Items() {
				t, ok := v.(*mdns.Target)
				if !ok {
					panic(fmt.Sprintf("cant handle %T", v))
				}
				if t.Marked {
					selected = append(selected, t)
				}
			}

			// if the user marked items, use only the marked
			// items.
			if len(selected) > 0 {
				m.Selected = selected
				return m, tea.Quit
			}

			// we reach here, if the user didnt mark any items
			// in that case, we select the item that was selected.
			s := m.list.SelectedItem()
			t, ok := s.(*mdns.Target)
			if !ok {
				panic(fmt.Sprintf("cant handle %T", s))
			}

			m.Selected = []*mdns.Target{t}
			return m, tea.Quit
		}
		if msg.String() == " " {
			s := m.list.SelectedItem()
			t, ok := s.(*mdns.Target)
			if !ok {
				panic(fmt.Sprintf("cant handle %T", s))
			}
			t.Marked = !t.Marked
			return m, cmd
		}
	case []*mdns.Target:
		i := make([]list.Item, 0)
		for _, v := range msg {
			i = append(i, v)
		}
		m.list.SetItems(i)
		return m, m.mdnsUpdates()
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

func (m *model) View() string {
	m.list.Title = fmt.Sprintf("Looking for devices %s", m.spinner.View())
	return docStyle.Render(m.list.View())
}
