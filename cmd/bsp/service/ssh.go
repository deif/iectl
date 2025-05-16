package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:       "ssh [enable|disable|status]",
	Short:     "Get ssh status or enable/disable",
	Args:      cobra.OnlyValidArgs,
	ValidArgs: []cobra.Completion{"enable", "disable", "status"},
	RunE: func(cmd *cobra.Command, args []string) error {
		// no arguments? - receive ssh status
		if len(args) == 0 || args[0] == "status" {
			return getSshStatus(cmd, args)
		}

		targets := target.FromContext(cmd.Context())
		for _, target := range targets {
			u := url.URL{
				Scheme: "https",
				Host:   target.Hostname,
				Path:   "/bsp/service/ssh",
			}

			var enable bool
			if args[0] == "enable" {
				enable = true
			}

			reqStruct := struct {
				Running bool `json:"running"`
			}{
				Running: enable,
			}

			body, err := json.Marshal(reqStruct)
			if err != nil {
				return fmt.Errorf("%s: unable to marshal request body: %w", target.Hostname, err)
			}

			req, err := http.NewRequest("PUT", u.String(), bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("%s: unable to create http put request: %w", target.Hostname, err)
			}

			resp, err := target.Client.Do(req)
			if err != nil {
				return fmt.Errorf("%s: unable to http post: %w", target.Hostname, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("%s: unexpected http status code: %d", target.Hostname, resp.StatusCode)
			}
			asJson, _ := cmd.Flags().GetBool("json")
			if !asJson {
				fmt.Printf("%s: 200 OK", target.Hostname)
				fmt.Println()
			}
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(sshCmd)
}

func getSshStatus(cmd *cobra.Command, _ []string) error {
	targets := target.FromContext(cmd.Context())
	for _, target := range targets {
		u := url.URL{
			Scheme: "https",
			Host:   target.Hostname,
			Path:   "/bsp/service/ssh",
		}

		resp, err := target.Client.Get(u.String())
		if err != nil {
			return fmt.Errorf("%s: unable to http get: %w", target.Hostname, err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
		default:
			return fmt.Errorf("%s: unexpected statuscode: %d", target.Hostname, resp.StatusCode)
		}

		asJson, _ := cmd.Flags().GetBool("json")
		if asJson {
			_, err = io.Copy(os.Stdout, resp.Body)
			if err != nil {
				return fmt.Errorf("%s: unable to copy to stdout: %w", target.Hostname, err)
			}
			return nil
		}

		dec := json.NewDecoder(resp.Body)
		response := &struct {
			Running bool `json:"running"`
		}{}

		err = dec.Decode(response)
		if err != nil {
			return fmt.Errorf("%s: unable to unmarshal response: %w", target.Hostname, err)
		}

		if response.Running {
			fmt.Printf("%s: SSH Service: enabled", target.Hostname)
		} else {
			fmt.Printf("%s: SSH Service: disabled", target.Hostname)
		}
		fmt.Println()
	}
	return nil
}
