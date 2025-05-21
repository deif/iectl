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

func addPrefixIfMissing(s string, prefix string) string {
	if !strings.HasPrefix(s, prefix) {
		return prefix + s
	} else {
		return s
	}
}

var (
	httpMethod string
	path       string
	interactive bool
)

func exec_method(method string, cmd *cobra.Command, args []string) error {
	httpMethod = method
	targets := target.FromContext(cmd.Context())
	if len(targets) > 1 {
		return fmt.Errorf("refusing to debug request with more than 1 target")
	}
	target := targets[0]

	if len(args) == 0 {
		return fmt.Errorf("missing required argument path")
	}

	reqBody := getBody(args[1:])

	interactive, _ = cmd.Flags().GetBool("interactive")

	path = args[0]
	addPrefixIfMissing(path, "/")
	addPrefixIfMissing(path, "/bsp")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasPrefix(path, "/bsp") {
		path = "/bsp" + path
	}

	urlString := "https://" + target.Hostname + path
	u, err := url.Parse(urlString)
	if err != nil {
		return fmt.Errorf("URL could not be parsed, reason %w", err)
	}

	fmt.Println(u.String())
	fmt.Println(u.RawQuery)

	req, err := http.NewRequest(method, u.String(), bytes.NewReader([]byte(reqBody)))
	if err != nil {
		return fmt.Errorf("unable to create http request: %w", err)
	}

	resp, err := target.Client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to http %s: %w", method, err)
	}
	defer resp.Body.Close()

	return formatOutput(resp)
}

func formatOutput(resp *http.Response) error {

	showBody := true
	if interactive {
		fmt.Printf("Request %s to %s completed with status code %d.\n", httpMethod, path, resp.StatusCode)
		if resp.Header.Get("Content-Type") == "application/zip" {
			fmt.Println("Response indicates a binary Content-Type, do you still want to print it? [y/N]")
			var ans string
			fmt.Scanln(&ans)
			if !strings.HasPrefix(strings.ToLower(ans), "y") {
				showBody = false
			}
		}
	} else {
		fmt.Printf("statusCode: %d\n", resp.StatusCode)
		fmt.Print("body: ")
	}
	if interactive && resp.Header.Get("Content-Length") == "0" {
		return nil
	}
	if showBody {
		_, err := io.Copy(os.Stdout, resp.Body)
		if err != nil {
			return fmt.Errorf("error reading http response: %w", err)
		}
	} else {
		fmt.Printf("<binary data, Content-Length %s>", resp.Header.Get("Content-Type"))
	}
	return nil
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Compose GET request",
	Args:  cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exec_method("GET", cmd, args)
	},
}
var postCmd = &cobra.Command{
	Use:   "post",
	Short: "Compose POST request",
	Args:  cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exec_method("POST", cmd, args)
	},
}
var putCmd = &cobra.Command{
	Use:   "put",
	Short: "Compose PUT request",
	Args:  cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exec_method("PUT", cmd, args)
	},
}
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Compose DELETE request",
	Args:  cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exec_method("DELETE", cmd, args)
	},
}

func init() {
	RootCmd.AddCommand(getCmd)
	RootCmd.AddCommand(postCmd)
	RootCmd.AddCommand(putCmd)
	RootCmd.AddCommand(deleteCmd)

}
