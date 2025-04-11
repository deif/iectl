package service

import (
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "service",
	Short: "Collection of service commands",
}
