package bsp

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

type AuthenticatedClientCtx string

var aClientKey AuthenticatedClientCtx = "github.com/deif/iectl/cmd/bsp.AuthenticatedClient"

var RootCmd = &cobra.Command{
	Use:   "bsp",
	Short: "Collection of commands relating to the bsp rest api",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		user, _ := cmd.Flags().GetString("username")
		pass, _ := cmd.Flags().GetString("password")

		host, _ := cmd.Flags().GetString("hostname")
		if host == "" {
			return fmt.Errorf("required flag \"hostname\" not set")
		}

		insecure, _ := cmd.Flags().GetBool("insecure")

		c, err := authenticatedClient(host, user, pass, insecure)
		if err != nil {
			return fmt.Errorf("unable to authenticate: %w", err)
		}

		cmd.SetContext(context.WithValue(cmd.Context(), aClientKey, c))
		return nil
	},
}

func init() {
	RootCmd.PersistentFlags().StringP("hostname", "t", "", "specify hostname or address to target")
	RootCmd.MarkPersistentFlagRequired("hostname")

	RootCmd.PersistentFlags().StringP("username", "u", "admin", "specify username")
	RootCmd.PersistentFlags().StringP("password", "p", "admin", "specify username")
	RootCmd.PersistentFlags().Bool("insecure", false, "do not verify connection certificates")
}
