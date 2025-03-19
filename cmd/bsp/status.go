package bsp

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "General system status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := cmd.Context().Value(aClientKey).(*http.Client)
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/system/status",
		}

		resp, err := client.Get(u.String())
		if err != nil {
			return fmt.Errorf("unable to http get: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected statuscode: %d", resp.StatusCode)
		}

		_, err = io.Copy(os.Stdin, resp.Body)
		if err != nil {
			return fmt.Errorf("unable to copy reponse body to stdout: %w", err)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
