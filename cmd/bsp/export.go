package bsp

import (
	"fmt"
	"strings"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export targets and credentials",
	Long: `Export the variables and targets in a format that can be programmatically evaluated.

The exported values include IECTL_BSP_TARGET, IECTL_BSP_USERNAME, and IECTL_BSP_PASSWORD.

If these values are set in the environment where iectl is executed,
iectl can automatically determine the appropriate targets.
This eliminates the need to specify the --target option with each
command or to rely on mDNS for target discovery.

This approach provides a more reliable and deterministic way to specify
exactly which targets should be used, avoiding the unpredictability
that can sometimes occur with mDNS.

Examples:

  Save all currently responding targets into the environment:

    $ eval $(iectl bsp export --target-all)
`,
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
