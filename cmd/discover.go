package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deif/iectl/mdns"

	"github.com/miekg/dns"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "discover deif devices on the network",
	Long: `discover deif devices on the network

Continuously scans and reports discovered devices in real time.
Default: Displays each discovered host only once, writing a new line as new hosts appear.
 --json: Emits the full list of all discovered devices as a JSON array every time a new device is found.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn("_base-unit-deif._tcp.local"), dns.TypePTR)

		browser := mdns.Browser{Question: *msg}
		timeout, _ := cmd.Flags().GetDuration("timeout")
		ctx := context.Background()
		if timeout != 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		updates, err := browser.Run(ctx)
		if err != nil {
			return fmt.Errorf("unable to browse mdns: %w", err)
		}

		// if running with json output, just dump
		// everthing from the browser.
		asJson, _ := cmd.Flags().GetBool("json")
		if asJson {
			for {
				u, ok := <-updates
				if !ok {
					break
				}
				p, err := json.Marshal(u)
				if err != nil {
					return fmt.Errorf("unable to marshal json: %w", err)
				}

				fmt.Println(string(p))
			}
		}

		known := make(map[string]struct{})
		for {
			u, ok := <-updates
			if !ok {
				break
			}
			for _, v := range u {
				_, exists := known[v.Hostname]
				if exists {
					continue
				}

				fmt.Println(v.Hostname)
				known[v.Hostname] = struct{}{}
			}
		}

		return nil
	},
}

func init() {
	discoverCmd.Flags().Duration("timeout", 0, "timeout, zero-value disables timeout")
	rootCmd.AddCommand(discoverCmd)

}
