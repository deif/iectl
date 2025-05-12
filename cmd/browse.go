package cmd

import (
	"context"
	"fmt"

	"github.com/deif/iectl/mdns"
	"github.com/deif/iectl/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/miekg/dns"
	openBrowser "github.com/pkg/browser"
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
		q := dns.Question{Name: dns.Fqdn("_base-unit-deif._tcp.local"), Qtype: dns.TypePTR}
		browser := mdns.Browser{Question: q}

		var err error
		updates, err = browser.Run(context.Background())
		if err != nil {
			return fmt.Errorf("unable to browse mdns: %w", err)
		}

		m := tui.BrowserModel(updates)

		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}

		// m.Selected should now hold whatever the user wanted to open
		// it may be empty, in case that the user just wanted to quit
		for _, v := range m.Selected {
			err := openBrowser.OpenURL(v.Description())
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(browseCmd)
}
