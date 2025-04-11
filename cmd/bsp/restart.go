package bsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/deif/iectl/auth"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:     "restart",
	Short:   "Reboots the device",
	Aliases: []string{"reboot"},
	RunE: func(cmd *cobra.Command, args []string) error {
		client := auth.FromContext(cmd.Context())
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/system/restart",
		}

		//TODO: accept delay as argument to restart - default to zero
		req := struct {
			Delay int `json:"delay"`
		}{
			Delay: 0,
		}

		body, err := json.Marshal(req)
		if err != nil {
			return err
		}

		resp, err := client.Post(u.String(), "application/json", bytes.NewReader(body))
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
	RootCmd.AddCommand(restartCmd)
}
