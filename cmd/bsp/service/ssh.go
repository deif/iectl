package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/deif/iectl/auth"
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

		client := auth.FromContext(cmd.Context())
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
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
			return fmt.Errorf("unable to marshal request body: %w", err)
		}

		req, err := http.NewRequest("PUT", u.String(), bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("unable to create http put request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("unable to http post: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
		}
		asJson, _ := cmd.Flags().GetBool("json")
		if !asJson {
			fmt.Println("Device answers: 200 OK")
		}

		// nothing for --json. users must deal with the exit code of iecli
		return nil
	},
}

func init() {
	RootCmd.AddCommand(sshCmd)
}

func getSshStatus(cmd *cobra.Command, args []string) error {
	client := auth.FromContext(cmd.Context())
	host, _ := cmd.Flags().GetString("hostname")
	u := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/bsp/service/ssh",
	}

	resp, err := client.Get(u.String())
	if err != nil {
		return fmt.Errorf("unable to http get: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	default:
		return fmt.Errorf("unexpected statuscode: %d", resp.StatusCode)
	}

	asJson, _ := cmd.Flags().GetBool("json")
	if asJson {
		_, err = io.Copy(os.Stdout, resp.Body)
		if err != nil {
			return fmt.Errorf("unable to copy to stdout: %w", err)
		}
		return nil
	}

	dec := json.NewDecoder(resp.Body)
	response := &struct {
		Running bool `json:"running"`
	}{}

	err = dec.Decode(response)
	if err != nil {
		return fmt.Errorf("unable to unmarshal response: %w", err)
	}

	if response.Running {
		fmt.Println("SSH Service: enabled")
	} else {
		fmt.Println("SSH Service: disabled")
	}

	return nil
}
