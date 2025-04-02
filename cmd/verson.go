package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	version = "n/a"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version info on iectl",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("iectl %s, commit %s, built at %s\n", version, commit, date)
		fmt.Printf("interactive terminal: %t\n", term.IsTerminal(int(os.Stdout.Fd())))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
