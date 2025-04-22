package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
		interactive, _ := cmd.Flags().GetBool("interactive")
		fmt.Printf("iectl %s, commit %s (%s)\n", version, commit, date)
		fmt.Printf("interactive terminal: %t\n", interactive)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
