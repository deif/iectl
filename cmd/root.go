package cmd

import (
	"os"

	"github.com/deif/iectl/cmd/bsp"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	rootCmd.PersistentFlags().BoolP(
		"interactive", "i", term.IsTerminal(int(os.Stdout.Fd())),
		"interactive mode, ask for passwords, display pretty ascii")
}
