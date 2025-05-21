package debug

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"net/http"
	"net/url"

	"github.com/deif/iectl/target"
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
		targets := target.FromContext(cmd.Context())
		if len(targets) > 1 {
			return fmt.Errorf("refusing to debug request with more than 1 target")
		}
		target := targets[0]

		quiet := false

		if len(args) == 0 {
			return fmt.Errorf("missing required argument path")
		}

		reqBody := getBody(args[1:])

		interactive, _ := cmd.Flags().GetBool("interactive")

		path := args[0]
		u := url.URL{
			Scheme: "https",
			Host:   target.Hostname,
			Path:   "/bsp/" + path,
		}

		req, err := http.NewRequest("GET", u.String(), bytes.NewReader([]byte(reqBody)))
		if err != nil {
			return fmt.Errorf("unable to create http request: %w", err)
		}

		resp, err := target.Client.Do(req)
		if err != nil {
			return fmt.Errorf("unable to http get: %w", err)
		}
		defer resp.Body.Close()

		if !quiet {
			fmt.Printf("Request GET to %s completed.\n", path)
		}

		showBody := true
		if interactive {
			if resp.Header.Get("Content-Type") == "application/zip" {
				fmt.Println("Response indicates a binary Content-Type, do you still want to print it? [y/N]")
				var ans string
				fmt.Scanln(&ans)
				if !strings.HasPrefix(strings.ToLower(ans), "y") {
					showBody = false
				}
			}
		}

		fmt.Println("status-code: ", resp.StatusCode)
		fmt.Print("body: ")
		if showBody {
			_, err := io.Copy(os.Stdout, resp.Body)
			if err != nil {
				return fmt.Errorf("error reading http response: %w", err)
			}
		} else {
			fmt.Printf("<binary data, Content-Length %s>", resp.Header.Get("Content-Type"))
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(getCmd)
}
