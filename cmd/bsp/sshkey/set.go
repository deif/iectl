package sshkey

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/deif/iectl/auth"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "set ssh public key(s) for the root user",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := auth.FromContext(cmd.Context())
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/keys/ssh",
		}

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

		resp, err := client.Post(u.String(), "application/json", bytes.NewReader(jsonPayload))
		if err != nil {
			return fmt.Errorf("unable to http post: %w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
		case http.StatusAccepted:
		case http.StatusBadRequest:
			return fmt.Errorf("bad SSH key, server responded with 400 Bad Request")
		default:
			return fmt.Errorf("unexpected statuscode: %d", resp.StatusCode)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(setCmd)
}
