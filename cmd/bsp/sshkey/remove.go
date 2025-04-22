package sshkey

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/deif/iectl/auth"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"delete"},
	Short:   "remove ssh public key(s) for the root user",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := auth.FromContext(cmd.Context())
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/keys/ssh",
		}

		req, err := http.NewRequest("DELETE", u.String(), nil)
		if err != nil {
			return fmt.Errorf("unable to create http request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("unable to http post: %w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
		case http.StatusAccepted:
		default:
			return fmt.Errorf("unexpected statuscode: %d", resp.StatusCode)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(removeCmd)
}
