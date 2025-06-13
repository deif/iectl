package bsp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	maxWidth = 60
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
var errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6262")).Render

type hostProgress struct {
	name     string
	status   string
	err      string
	progress progress.Model
}

type multiProgressModel struct {
	hosts     map[string]hostProgress
	hostOrder []string

	quitting bool
}

type multiProgressModelPleaseQuit struct{}

type hostUpdate struct {
	progress progressMsg
	host     string
}

func multiProgressModelWithTargets(t []*firmwareTarget) (multiProgressModel, error) {
	mpModel := multiProgressModel{hosts: make(map[string]hostProgress)}
	var keys []string
	for _, v := range t {
		keys = append(keys, v.Hostname)
		_, exists := mpModel.hosts[v.Hostname]
		if exists {
			return multiProgressModel{}, fmt.Errorf("%s is not unique", v.Hostname)
		}
		p := progress.New(progress.WithGradient("#004637", "#12bc00"))

		mpModel.hosts[v.Hostname] = hostProgress{
			name:     v.Hostname,
			status:   "Queued...",
			progress: p,
		}
	}

	sort.Strings(keys)
	mpModel.hostOrder = keys

	return mpModel, nil
}

func (m multiProgressModel) Init() tea.Cmd {
	return nil
}

// Update handles UI updates
// you might be tempted to ask the question, why are these not
// pointer receivers instead of having to deal with all these
// mutated values, thing is: bubbletea gets racy if not done this way
func (m multiProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyCtrlC:
			return m, tea.Quit
		}
		return m, nil

	case tea.WindowSizeMsg:
		for i, v := range m.hosts {
			v.progress.Width = msg.Width
			if v.progress.Width > maxWidth {
				v.progress.Width = maxWidth
			}

			// put state back
			m.hosts[i] = v
		}
		return m, nil

	case multiProgressModelPleaseQuit:
		m.quitting = true
		return m, nil

	case hostUpdate:
		t := m.hosts[msg.host]
		t.status = msg.progress.status
		t.err = msg.progress.err
		cmd := t.progress.SetPercent(msg.progress.ratio)

		// put mutated state back
		m.hosts[msg.host] = t

		return m, cmd

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		cmds := make([]tea.Cmd, 0)
		var animating bool
		for i := range m.hosts {
			h := m.hosts[i]
			if h.progress.IsAnimating() {
				animating = true
			}

			prog, cmd := h.progress.Update(msg)
			if cmd == nil {
				// this FrameMsg was not for this progressbar
				continue
			}

			// we have mutated state, but values back
			progModel := prog.(progress.Model)
			h.progress = progModel
			m.hosts[i] = h

			cmds = append(cmds, cmd)
		}

		if m.quitting && !animating {
			return m, tea.Quit
		}

		return m, tea.Batch(cmds...)

	default:
		return m, nil
	}
}

const hostnamePad = 18

func (m multiProgressModel) View() string {
	view := "Installing firmware..." + "\n\n"
	var h hostProgress
	for _, v := range m.hostOrder {
		h = m.hosts[v]

		view += formatHostname(h.name, hostnamePad) + " " + h.progress.View() + "\n" +
			strings.Repeat(" ", hostnamePad+1)

		if h.err != "" {
			view += errorStyle(h.err)

		} else {
			view += helpStyle(h.status)
		}

		view += "\n\n"
	}
	return view
}
func formatHostname(text string, maxLen int) string {
	if len(text) == maxLen {
		return text
	}
	if len(text) < maxLen {
		return strings.Repeat(" ", maxLen-len(text)) + text
	}
	return "..." + text[len(text)-maxLen+3:]
}
