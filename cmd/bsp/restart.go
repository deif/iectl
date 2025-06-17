package bsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:     "restart",
	Short:   "Reboots the device",
	Aliases: []string{"reboot"},
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := target.FromContext(cmd.Context())
		for _, target := range targets {
			u := url.URL{
				Scheme: "https",
				Host:   target.Hostname,
				Path:   "/bsp/system/restart",
			}

			delay, _ := cmd.Flags().GetDuration("delay")
			req := struct {
				Delay int `json:"delay"`
			}{
				// nearest second
				Delay: int(math.Ceil(delay.Seconds())),
			}

			body, err := json.Marshal(req)
			if err != nil {
				return err
			}

			resp, err := target.Client.Post(u.String(), "application/json", bytes.NewReader(body))
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
		}
		return nil
	},
}

func init() {
	restartCmd.Flags().Duration("delay", 0, "restart delay")
	RootCmd.AddCommand(restartCmd)
}
