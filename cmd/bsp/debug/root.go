package debug

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

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
	path        string
	interactive bool
	printHeader bool
	printBinary bool
)

func execMethod(method string, cmd *cobra.Command, args []string) error {
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

func mimeIsBinary(mime string) bool {
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

func isResponsePrintable(read *bufio.Reader) bool {
	bytes, err := read.Peek(512)
	if err == io.EOF {
		return utf8.Valid(bytes)
	}
	if err != nil {
		return false
	}

	fmt.Printf("%d %b\n", len(bytes), bytes)

	// attempt to read utf8 runes one-by-one.
	i := 0
	for i < len(bytes) {
		rune, size := utf8.DecodeRune(bytes[i:])
		fmt.Printf("%s\n", rune)
		if rune == utf8.RuneError {
			// found unreadable byte.
			// if we are at the end of the chunk, see if last byte(s) could
			// be the start of valid utf8 that got cut off
			// utf8 maxes at 4 octets per rune
			if i < len(bytes)-4 {
				for _, byte := range bytes[i:] {
					if !utf8.RuneStart(byte) {
						return false
					}
				}
			} else {
				// non-utf8 byte found and it cannot have been cut off
				// by chunking. thus conclude binary data
				return false
			}
		}
		i += size
	}

	return true
}

func formatOutput(resp *http.Response) error {
	showBody := true
	body := bufio.NewReader(resp.Body)
	if interactive && !printBinary && (mimeIsBinary(resp.Header.Get("Content-Type")) || !isResponsePrintable(body)) {
		showBody = false
	}
	head, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return fmt.Errorf("malformed HTTP response: %w", err)
	}
	fmt.Printf("%s", head)
	if showBody {
		_, err := io.Copy(os.Stdout, body)
		if err != nil {
			return fmt.Errorf("error reading http response: %w", err)
		}
	} else {
		fmt.Printf("<binary data of type %s>\n", resp.Header.Get("Content-Type"))
		fmt.Println("(use --print-binary option to print the output anyway)")
	}
	return nil
}

var getCmd = &cobra.Command{
	Use:     "get",
	Aliases: []string{"GET"},
	Short:   "Compose GET request",
	Args:    cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return execMethod("GET", cmd, args)
	},
}
var postCmd = &cobra.Command{
	Use:     "post",
	Aliases: []string{"POST"},
	Short:   "Compose POST request",
	Args:    cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return execMethod("POST", cmd, args)
	},
}
var putCmd = &cobra.Command{
	Use:     "put",
	Aliases: []string{"PUT"},
	Short:   "Compose PUT request",
	Args:    cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return execMethod("PUT", cmd, args)
	},
}
var deleteCmd = &cobra.Command{
	Use:     "delete",
	Aliases: []string{"DELETE"},
	Short:   "Compose DELETE request",
	Args:    cobra.OnlyValidArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return execMethod("DELETE", cmd, args)
	},
}

func init() {
	RootCmd.Flags().BoolVar(&printHeader, "header", false, "If true, print HTTP headers of response.")
	RootCmd.Flags().BoolVar(&printBinary, "print-binary", false, "If set, print binary output even in interactive mode.")
	RootCmd.AddCommand(getCmd)
	RootCmd.AddCommand(postCmd)
	RootCmd.AddCommand(putCmd)
	RootCmd.AddCommand(deleteCmd)

}
