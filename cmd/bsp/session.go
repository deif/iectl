package bsp

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var (
	export       bool
	exportToFile string
)

func init() {
	sessionCmd.Flags().BoolVar(&export, "export", false, "sourceable variables written to stdout")
	sessionCmd.Flags().StringVar(&exportToFile, "export-to-file", "", "create or overwrite sourceable file")
	RootCmd.AddCommand(sessionCmd)
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manages session, targets and variables",
	Long: `Manage sessions in various ways, the simplest being exporting IECTL_* environment variables.

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

    $ source <(iectl bsp session --export --target-all)

  Browse for targets and save a file that can be sourced later:

    $ iectl bsp session --export-to-file my-particular-targets
	$ source my-particular-targets
  
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := target.FromContext(cmd.Context())
		var t = make([]string, 0)
		for _, target := range targets {
			t = append(t, target.Hostname)
		}

		fds := make([]io.Writer, 0)
		if export {
			fds = append(fds, os.Stdout)
		}

		if exportToFile != "" {
			fd, err := os.Create(exportToFile)
			if err != nil {
				return fmt.Errorf("unable to create file %s: %w", exportToFile, err)
			}
			fds = append(fds, fd)
		}

		writer := io.MultiWriter(fds...)

		// TODO: we should properly support different passwords
		//		 for different devices...
		// TODO: at some point, we should properly store refresh tokens somewhere, and
		//       and then throw away password.
		fmt.Fprintf(writer, "export IECTL_BSP_TARGET=%q\n", strings.Join(t, ","))

		user, _ := cmd.Flags().GetString("username")
		fmt.Fprintf(writer, "export IECTL_BSP_USERNAME=%q\n", user)

		password, _ := cmd.Flags().GetString("password")
		fmt.Fprintf(writer, "export IECTL_BSP_PASSWORD=%q\n", password)

		// close things that can be closed
		for _, v := range fds {
			closer, ok := v.(io.Closer)
			if ok {
				closer.Close()
			}
		}

		return nil
	},
}
