package debug

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:    "debug",
	Hidden: true,
	Short:  "Generic methods for composing nonspecific HTTP requests for testing and debugging.",
	Long: `Do a single HTTP request (GET, POST, PUT, or DELETE) of specified method to an endpoint with optional additional request body or query parameters.
Handling of request and response content is minimal. Multi-target mode is _not_ supported. Primarily intended for debugging and testing of REST endpoints during development.

Example usages:
- iectl bsp --target iE250-05eb2f.local debug get /hostname
- iectl bsp --target iE250-05eb2f.local debug post /service/ssh '{"running": false}' --header
- iectl bsp --target iE250-05eb2f.local debug get "/system/log?limit=5"
`,
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
	path         string
	interactive  bool
	printHeader  bool
	noWarnBinary bool
)

func exec_method(method string, cmd *cobra.Command, args []string) error {
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

func is_binary_mime_best_effort(mime string) bool {
	for _, exact := range [...]string{"application/zip", "application/octet-stream", "application/pdf"} {
		if exact == mime {
			return true
		}
	}
	for _, prefix := range [...]string{"image/", "audio/", "video/"} {
		if strings.HasPrefix(mime, prefix) {
			return true
		}
	}
	return false
}

func formatOutput(resp *http.Response) error {
	showBody := true
	if interactive && !noWarnBinary && is_binary_mime_best_effort(resp.Header.Get("Content-Type")) {
		fmt.Println("Response indicates a binary Content-Type, do you still want to print it? [y/N]")
		var ans string
		fmt.Scanln(&ans)
		if !strings.HasPrefix(strings.ToLower(ans), "y") {
			showBody = false
		}
	}
	head, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return fmt.Errorf("malformed HTTP response: %w", err)
	}
	fmt.Printf("%s", head)
	if showBody {
		_, err := io.Copy(os.Stdout, resp.Body)
		if err != nil {
			return fmt.Errorf("error reading http response: %w", err)
		}
	} else {
		fmt.Printf("<binary data of type %s, length %s>", resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"))
	}
	return nil
}

var getCmd = &cobra.Command{
	Use:     "get",
	Aliases: []string{"GET"},
	Short:   "Compose GET request",
	Args:    cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exec_method("GET", cmd, args)
	},
}
var postCmd = &cobra.Command{
	Use:     "post",
	Aliases: []string{"POST"},
	Short:   "Compose POST request",
	Args:    cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exec_method("POST", cmd, args)
	},
}
var putCmd = &cobra.Command{
	Use:     "put",
	Aliases: []string{"PUT"},
	Short:   "Compose PUT request",
	Args:    cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exec_method("PUT", cmd, args)
	},
}
var deleteCmd = &cobra.Command{
	Use:     "delete",
	Aliases: []string{"DELETE"},
	Short:   "Compose DELETE request",
	Args:    cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exec_method("DELETE", cmd, args)
	},
}

func init() {
	RootCmd.Flags().BoolVar(&printHeader, "header", false, "If true, print HTTP headers of response.")
	RootCmd.Flags().BoolVar(&noWarnBinary, "no-warn-binary", false, "If false and in interactive mode, do not warn about http response containing binary data.")
	RootCmd.AddCommand(getCmd)
	RootCmd.AddCommand(postCmd)
	RootCmd.AddCommand(putCmd)
	RootCmd.AddCommand(deleteCmd)

}
