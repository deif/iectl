package bsp

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/deif/iectl/auth"
	"github.com/deif/iectl/cmd/bsp/service"
	"github.com/deif/iectl/cmd/bsp/sshkey"
	"github.com/deif/iectl/target"
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

		targets, _ := cmd.Flags().GetStringSlice("target")
		if len(targets) == 0 {
			return fmt.Errorf("required flag \"target\" not set")
		}

		interactive, _ := cmd.Flags().GetBool("interactive")

		collection := target.Collection{}

		for _, host := range targets {
			c, err := auth.Client(host, user, pass, insecure)

			// If we have a terminal, and the error was invalid credentials
			// try to fix the issue by asking for another password...
			if errors.Is(err, auth.ErrInvalidCredentials) && interactive {
				for {
					fmt.Printf("Enter password for %s@%s: ", user, host)

					var p []byte
					p, err = readPassword()
					if err != nil {
						return fmt.Errorf("unable to ask for password: %w", err)
					}

					// we might as well try this newly entered password
					// on future targets aswell
					pass = string(p)

					fmt.Println()
					c, err = auth.Client(host, user, pass, insecure)
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

			collection = append(collection, target.Endpoint{Hostname: host, Client: c})

		}

		cmd.SetContext(target.NewContext(cmd.Context(), collection))

		return nil
	},
}

func init() {
	RootCmd.PersistentFlags().StringSliceP("target", "t", []string{}, "specify hostname(s) or address(es) to target(s)")
	RootCmd.MarkPersistentFlagRequired("target")

	RootCmd.PersistentFlags().StringP("username", "u", "admin", "specify username")
	RootCmd.PersistentFlags().StringP("password", "p", "admin", "specify username")
	RootCmd.PersistentFlags().Bool("insecure", false, "do not verify connection certificates")
	RootCmd.AddCommand(service.RootCmd)
	RootCmd.AddCommand(sshkey.RootCmd)
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
