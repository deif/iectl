package bsp

import (
	"fmt"
	"strings"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "export variables and targets",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := target.FromContext(cmd.Context())
		var t = make([]string, 0)
		for _, target := range targets {
			t = append(t, target.Hostname)
		}

		// TODO: we should properly support different passwords
		//		 for different devices...
		fmt.Printf("export IECTL_BSP_TARGET=%q\n", strings.Join(t, ","))

		user, _ := cmd.Flags().GetString("username")
		fmt.Printf("export IECTL_BSP_USERNAME=%q\n", user)

		password, _ := cmd.Flags().GetString("password")
		fmt.Printf("export IECTL_BSP_PASSWORD=%q\n", password)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(exportCmd)
}
