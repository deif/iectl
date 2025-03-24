package bsp

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var firmwareCmd = &cobra.Command{
	Use:   "install",
	Short: "Install new firmware on device",
	Args:  cobra.MatchAll(cobra.ExactArgs(1)),
	RunE: func(cmd *cobra.Command, args []string) error {
		fd, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("unable to open \"%q\": %w", args[0], err)
		}
		defer fd.Close()

		info, err := fd.Stat()
		if err != nil {
			return fmt.Errorf("unable to stat \"%q\": %w", args[0], err)
		}

		client := cmd.Context().Value(aClientKey).(*http.Client)
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/firmware/file",
		}

		r, w, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("unable to create pipe: %w", err)
		}

		mp := multipart.NewWriter(w)
		multipartWriter, err := mp.CreateFormFile("file", filepath.Base(args[0]))

		if err != nil {
			return fmt.Errorf("unable to create multipart body: %s", err)
		}

		go func() {
			_, err := io.Copy(multipartWriter, fd)
			if err != nil {
				panic("io.Copy failed: " + err.Error())
			}

			mp.Close()
			w.Close()
		}()

		req, err := http.NewRequest("POST", u.String(), r)
		if err != nil {
			return fmt.Errorf("unable to create http request: %w", err)
		}
		req.Header.Set("Content-Type", mp.FormDataContentType())

		// We happen to know that the multipart writers various headers and stuff
		// is usually 232 bytes long - given that the content-type and fieldname dosnt change
		// - all we need is add the actual file size and size of the filename
		// what could go wrong ? - off the top of my head, the only solid approach would
		// be to send the request to /dev/null first and count the bytes - but lets see
		// if we can get by using this constant...
		req.ContentLength = 232 + info.Size() + int64(len(filepath.Base(args[0])))

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("unable to http post: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(firmwareCmd)
}
