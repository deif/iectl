package sshkey

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "set ssh public key(s) for the root user",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var keymaterial strings.Builder
		for _, v := range args {
			c, err := os.ReadFile(v)
			if err != nil {
				return fmt.Errorf("unable to read %s: %w", v, err)
			}

			if len(c) == 0 {
				return fmt.Errorf("contents of %s is empty", v)
			}

			keymaterial.Write(c)

			if c[len(c)-1] != '\n' {
				keymaterial.WriteRune('\n')
			}
		}

		payload := struct {
			Certificate string `json:"certificate"`
		}{
			Certificate: keymaterial.String(),
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("unable to marshal json payload: %w", err)
		}

		targets := target.FromContext(cmd.Context())
		for _, target := range targets {
			u := url.URL{
				Scheme: "https",
				Host:   target.Hostname,
				Path:   "/bsp/keys/ssh",
			}

			resp, err := target.Client.Post(u.String(), "application/json", bytes.NewReader(jsonPayload))
			if err != nil {
				return fmt.Errorf("%s: unable to http post: %w", target.Hostname, err)
			}
			defer resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusOK:
			case http.StatusAccepted:
			case http.StatusBadRequest:
				return fmt.Errorf("%s: bad SSH key, server responded with 400 Bad Request", target.Hostname)
			default:
				return fmt.Errorf("%s: unexpected statuscode: %d", target.Hostname, resp.StatusCode)
			}
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(setCmd)
}
