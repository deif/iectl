package sshkey

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"delete"},
	Short:   "remove ssh public key(s) for the root user",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := target.FromContext(cmd.Context())
		for _, target := range targets {
			u := url.URL{
				Scheme: "https",
				Host:   target.Hostname,
				Path:   "/bsp/keys/ssh",
			}

			req, err := http.NewRequest("DELETE", u.String(), nil)
			if err != nil {
				return fmt.Errorf("%s: unable to create http request: %w", target.Hostname, err)
			}

			resp, err := target.Client.Do(req)
			if err != nil {
				return fmt.Errorf("%s: unable to http post: %w", target.Hostname, err)
			}
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
			case http.StatusAccepted:
			default:
				return fmt.Errorf("%s: unexpected statuscode: %d", target.Hostname, resp.StatusCode)
			}
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(removeCmd)
}
