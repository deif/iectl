package sshkey

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "sshkey",
	Short: "Get, set or remove ssh public key(s) for the root user",
	Args:  cobra.MatchAll(cobra.NoArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := target.FromContext(cmd.Context())
		for _, target := range targets {
			u := url.URL{
				Scheme: "https",
				Host:   target.Hostname,
				Path:   "/bsp/keys/ssh",
			}

			resp, err := target.Client.Get(u.String())
			if err != nil {
				return fmt.Errorf("%s: unable to http post: %w", target.Hostname, err)
			}
			defer resp.Body.Close()

			asJson, _ := cmd.Flags().GetBool("json")

			switch resp.StatusCode {
			case http.StatusOK:
			case http.StatusNotFound:
				if asJson {
					fmt.Printf("\"%s %s\"\n", target.Hostname, "has no authorized_key.")
					continue
				}
				fmt.Println(target.Hostname, "has no authorized_key.")
				continue
			default:
				return fmt.Errorf("%s: unexpected statuscode: %d", target.Hostname, resp.StatusCode)
			}

			if asJson {
				_, err := io.Copy(os.Stdout, resp.Body)
				if err != nil {
					return err
				}
				continue
			}

			wrap := struct {
				Certificate string `json:"certificate"`
			}{}

			dec := json.NewDecoder(resp.Body)
			err = dec.Decode(&wrap)
			if err != nil {
				return fmt.Errorf("%s: unable to unmarshal json: %w", target.Hostname, err)
			}

			fmt.Print(wrap.Certificate)
		}
		return nil
	},
}

func init() {
}
