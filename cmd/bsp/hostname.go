package bsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var hostnameCmd = &cobra.Command{
	Use:   "hostname <new hostname>",
	Short: "Get or set hostname",
	RunE: func(cmd *cobra.Command, args []string) error {
		// no arguments? - receive hostname status
		if len(args) == 0 {
			return gethostnameStatus(cmd, args)
		}

		client := cmd.Context().Value(aClientKey).(*http.Client)
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/hostname",
		}

		reqStruct := struct {
			Hostname string `json:"hostname"`
		}{
			Hostname: args[0],
		}

		body, err := json.Marshal(reqStruct)
		if err != nil {
			return fmt.Errorf("unable to marshal request body: %w", err)
		}

		resp, err := client.Post(u.String(), "application/json", bytes.NewReader(body))
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
	RootCmd.AddCommand(hostnameCmd)
}

func gethostnameStatus(cmd *cobra.Command, args []string) error {
	client := cmd.Context().Value(aClientKey).(*http.Client)
	host, _ := cmd.Flags().GetString("hostname")
	u := url.URL{
		Scheme: "https",
		Host:   host,
		Path:   "/bsp/hostname",
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
		Hostname string `json:"hostname"`
	}{}

	err = dec.Decode(response)
	if err != nil {
		return fmt.Errorf("unable to unmarshal response: %w", err)
	}

	fmt.Printf("Current hostname: %s\n", response.Hostname)

	return nil
}
