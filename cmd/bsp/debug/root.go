package debug

import (
	"bytes"
	"fmt"
	"io"
	"unicode/utf8"

	"net/http"
	"net/url"

	"github.com/deif/iectl/auth"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "debug",
	Short: "Generic methods for composing nonspecific HTTP requests for testing and debugging.",
}

func getBody(args []string) string {
	body := ""
	for _, arg := range args {
		body += arg
	}
	return body
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Compose GET request",
	Args:  cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet := false
		unsafePrint := false
		client := auth.FromContext(cmd.Context())

		if len(args) == 0 {
			return fmt.Errorf("missing required argument path")
		}

		reqBody := getBody(args[1:])

		host, _ := cmd.Flags().GetString("hostname")
		interactive, _ := cmd.Flags().GetBool("interactive")

		path := args[0]
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/" + path,
		}

		req, err := http.NewRequest("GET", u.String(), bytes.NewReader([]byte (reqBody)))
		if err != nil {
			return fmt.Errorf("unable to create http request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("unable to http get: %w", err)
		}
		defer resp.Body.Close()

		if !quiet {
			fmt.Printf("Request GET to %s completed.\n", path)
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading http response: %w", err)
		}

		fmt.Println("status-code: ", resp.StatusCode)
		if !unsafePrint && !utf8.Valid(respBody) {
			fmt.Printf("body: <binary data, length %d>", len(respBody))
			if interactive {
				// TODO ask to save to file?
			}
		} else {
			fmt.Printf("body: %s\n", respBody)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(getCmd)
}
