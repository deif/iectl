package cmd

import (
	"os"

	"github.com/deif/iectl/cmd/bsp"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "iectl",
	Short: "cli for the DEIF Intelligent Energy generation of products.",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(bsp.RootCmd)
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
