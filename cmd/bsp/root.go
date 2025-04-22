package bsp

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/deif/iectl/auth"
	"github.com/deif/iectl/cmd/bsp/service"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var RootCmd = &cobra.Command{
	Use:   "bsp",
	Short: "Collection of commands relating to the bsp rest api",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		insecure, _ := cmd.Flags().GetBool("insecure")
		user, _ := cmd.Flags().GetString("username")
		pass, _ := cmd.Flags().GetString("password")

		host, _ := cmd.Flags().GetString("hostname")
		if host == "" {
			return fmt.Errorf("required flag \"hostname\" not set")
		}

		c, err := auth.Client(host, user, pass, insecure)

		interactive, _ := cmd.Flags().GetBool("interactive")

		// If we have a terminal, and the error was invalid credentials
		// try to fix the issue by asking for another password...
		if errors.Is(err, auth.ErrInvalidCredentials) && interactive {
			for {
				fmt.Printf("Enter password for \"%s\": ", user)

				var p []byte
				p, err = readPassword()
				if err != nil {
					return fmt.Errorf("unable to ask for password: %w", err)
				}

				fmt.Println()
				c, err = auth.Client(host, user, string(p), insecure)
				if errors.Is(err, auth.ErrInvalidCredentials) {
					continue
				}
				if err != nil {
					return fmt.Errorf("unable to authenticate: %w", err)
				}

				break
			}
		}

		if err != nil {
			return fmt.Errorf("unable to authenticate: %w", err)
		}

		cmd.SetContext(auth.NewContext(cmd.Context(), c))

		return nil
	},
}

func init() {
	RootCmd.PersistentFlags().StringP("hostname", "t", "", "specify hostname or address to target")
	RootCmd.MarkPersistentFlagRequired("hostname")

	RootCmd.PersistentFlags().StringP("username", "u", "admin", "specify username")
	RootCmd.PersistentFlags().StringP("password", "p", "admin", "specify username")
	RootCmd.PersistentFlags().Bool("insecure", false, "do not verify connection certificates")
	RootCmd.AddCommand(service.RootCmd)
}

func readPassword() ([]byte, error) {
	oldState, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		return nil, fmt.Errorf("error getting terminal state: %w", err)
	}

	// Handle interrupts to restore the terminal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		_, open := <-sigChan
		if !open {
			return
		}
		_ = term.Restore(int(os.Stdin.Fd()), oldState) // Restore before exiting
		fmt.Println("\nInterrupted! Terminal restored.")
		os.Exit(1)
	}()

	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return nil, fmt.Errorf("error reading password: %w", err)

	}

	signal.Stop(sigChan)
	close(sigChan)

	return password, nil

}
