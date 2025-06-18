package cmd

import (
	"log"
	"net/http"
	"os"

	"github.com/deif/iectl/cmd/bsp"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	_ "net/http/pprof"
)

var rootCmd = &cobra.Command{
	Use:   "iectl",
	Short: "cli for the IE generation of DEIF products.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		pprofEnable, _ := cmd.Flags().GetBool("enable-pprof")
		if pprofEnable {
			go func() {
				log.Printf("pprof listening on 0.0.0.0:6060")
				err := http.ListenAndServe("0.0.0.0:6060", nil)
				if err != nil {
					panic(err)
				}
			}()
		}

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// without this, it is impossible to enable the pprof debug endpoint
	// on commands that have PersistentPreRun's of their own.
	// https://github.com/spf13/cobra/pull/2044
	cobra.EnableTraverseRunHooks = true

	rootCmd.AddCommand(bsp.RootCmd)
	rootCmd.PersistentFlags().Bool("enable-pprof", false, "enable debug pprof server on 0.0.0.0:6060")
	rootCmd.PersistentFlags().BoolP("json", "j", false, "output as json")
	rootCmd.PersistentFlags().BoolP(
		"interactive", "i", term.IsTerminal(int(os.Stdout.Fd())),
		"interactive mode, ask for passwords, display pretty ascii")
}
