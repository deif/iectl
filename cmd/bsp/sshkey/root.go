package sshkey

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/deif/iectl/auth"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "sshkey",
	Short: "Get, set or remove ssh public key(s) for the root user",
	Args:  cobra.MatchAll(cobra.NoArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := auth.FromContext(cmd.Context())
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/user/admin/sshkey",
		}

		resp, err := client.Get(u.String())
		if err != nil {
			return fmt.Errorf("unable to http post: %w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
		case http.StatusNotFound:
			fmt.Println("Device has no authorized_key.")
			return nil
		default:
			return fmt.Errorf("unexpected statuscode: %d", resp.StatusCode)
		}

		asJson, _ := cmd.Flags().GetBool("json")
		if asJson {
			_, err := io.Copy(os.Stdout, resp.Body)
			return err
		}

		wrap := struct {
			Certificate string `json:"certificate"`
		}{}

		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&wrap)
		if err != nil {
			return fmt.Errorf("unable to unmarshal json: %w", err)
		}

		fmt.Print(wrap.Certificate)
		return nil
	},
}

func init() {
}
