package bsp

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/deif/iectl/auth"
	"github.com/spf13/cobra"
)

var factoryResetCmd = &cobra.Command{
	Use:   "factory-reset",
	Short: "Factory reset device",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := auth.FromContext(cmd.Context())
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/system/reset",
		}

		resp, err := client.Post(u.String(), "application/binary", nil)
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
	RootCmd.AddCommand(factoryResetCmd)
}
