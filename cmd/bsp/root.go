package bsp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deif/iectl/auth"
	"github.com/deif/iectl/cmd/bsp/service"
	"github.com/deif/iectl/cmd/bsp/sshkey"
	"github.com/deif/iectl/mdns"
	"github.com/deif/iectl/target"
	"github.com/deif/iectl/tui"
	"github.com/miekg/dns"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var RootCmd = &cobra.Command{
	Use:   "bsp",
	Short: "Collection of commands relating to the bsp rest api",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := cmd.ValidateRequiredFlags()
		if err != nil {
			return err
		}
		err = cmd.ValidateFlagGroups()
		if err != nil {
			return err
		}

		targets, err := targetsFromFlags(cmd)
		if err != nil {
			return fmt.Errorf("could not get targets from flags: %w", err)
		}

		insecure, _ := cmd.Flags().GetBool("insecure")
		user, _ := cmd.Flags().GetString("username")
		pass, _ := cmd.Flags().GetString("password")

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

func targetsFromFlags(cmd *cobra.Command) ([]string, error) {
	// if targets where directly specified, use them
	t, _ := cmd.Flags().GetStringSlice("target")
	if len(t) != 0 {
		return t, nil
	}

	timeout, _ := cmd.Flags().GetDuration("target-timeout")
	pickAny, _ := cmd.Flags().GetBool("target-any")
	if pickAny {
		return firstTarget(timeout)
	}

	pickAll, _ := cmd.Flags().GetBool("target-all")
	if pickAll {
		return allTargets(timeout)
	}

	// if we reached this far, there where no --target's specified
	// and --target-any and target-all where both off. If we have an interactive
	// terminal - let the user choose though the browser
	interactive, _ := cmd.Flags().GetBool("interactive")
	if interactive {
		return browseTargets()
	}

	return nil, fmt.Errorf("no targets specified, and terminal is not interactive")
}

func firstTarget(timeout time.Duration) ([]string, error) {
	q := dns.Question{Name: dns.Fqdn("_base-unit-deif._tcp.local"), Qtype: dns.TypePTR}
	browser := mdns.Browser{Question: q}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	updates, err := browser.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to browse mdns: %w", err)
	}

	t, ok := <-updates
	if !ok {
		return nil, fmt.Errorf("found no targets within deadline")
	}

	return []string{t[0].Hostname}, nil
}

func allTargets(timeout time.Duration) ([]string, error) {
	q := dns.Question{Name: dns.Fqdn("_base-unit-deif._tcp.local"), Qtype: dns.TypePTR}
	browser := mdns.Browser{Question: q}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	updates, err := browser.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to browse mdns: %w", err)
	}

	// we are looking for the last update.
	var found []*mdns.Target
	for {
		t, ok := <-updates
		if !ok {
			break
		}
		found = t
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("found no targets within deadline")
	}

	targets := make([]string, 0)
	for _, v := range found {
		targets = append(targets, v.Hostname)
	}

	return targets, nil
}

func browseTargets() ([]string, error) {
	q := dns.Question{Name: dns.Fqdn("_base-unit-deif._tcp.local"), Qtype: dns.TypePTR}

	browser := mdns.Browser{Question: q}

	var err error
	updates, err := browser.Run(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to browse mdns: %w", err)
	}

	m := tui.BrowserModel(updates)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return nil, fmt.Errorf("unable to run tui: %w", err)
	}

	targets := make([]string, 0)
	for _, v := range m.Selected {
		targets = append(targets, v.Hostname)
	}

	return targets, nil
}

func init() {
	RootCmd.PersistentFlags().StringSliceP("target", "t", []string{}, "specify hostname(s) or address(es) to target(s)")
	RootCmd.PersistentFlags().Bool("target-any", false, "any target, first answer picked - for networks with exactly one controller")
	RootCmd.PersistentFlags().Bool("target-all", false, "search for targets, operate on all found within timeout")
	RootCmd.MarkFlagsMutuallyExclusive("target", "target-any", "target-all")

	RootCmd.PersistentFlags().Duration("target-timeout", time.Second, "timeout for --target-all and --target-any")

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
