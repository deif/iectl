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
		fmt.Printf("iectl %s, commit %s, built at %s\n", version, commit, date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
