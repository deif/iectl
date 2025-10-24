package bsp

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"

	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
)

var (
	user             string
	synchronizePanes bool

	layoutVertical   bool
	layoutHorizontal bool
)

var ssh = &cobra.Command{
	Use:   "ssh",
	Short: "Open ssh sessions to one or many targets",
	RunE: func(cmd *cobra.Command, args []string) error {
		// TODO: work on targets - in this use-case they should
		// not try to authenticate as we have no use for an authenticated http client
		// - which gets annoying because we have to add --insecure
		//   even though no insecure communication needs to happen
		//
		// possible solutions
		//  * dont to the authentication before a target is acturally getting
		// 	  used to do http
		//  * maybe provide some sort of WithAuthentication option to FromContext
		//  * maybe time will show a nicer solution...
		targets := target.FromContext(cmd.Context())

		var (
			executable string
			eArgs      []string
			err        error
		)

		// if more then one target, jump into tmux
		if len(targets) > 1 {
			executable, err = exec.LookPath("tmux")
			if err != nil {
				return fmt.Errorf("tmux not found: %w", err)
			}

			eArgs = tmuxCommandFromTargets(targets)

			// if we only have one host, just exec directly to ssh
		} else {
			executable, err = exec.LookPath("ssh")
			if err != nil {
				return fmt.Errorf("ssh not found: %w", err)
			}

			eArgs = []string{"ssh", fmt.Sprintf("%s@%s", user, targets[0].Hostname)}
		}

		// replace current process with tmux
		err = syscall.Exec(executable, eArgs, os.Environ())
		if err != nil {
			return fmt.Errorf("exec failed: %w", err)
		}

		return nil
	},
}

func init() {
	ssh.Flags().StringVar(&user, "ssh-user", "root", "ssh username")
	ssh.Flags().BoolVar(&synchronizePanes, "synchronize-panes", true, "enable or disable pane synchronization, only applies with multiple targets")

	ssh.Flags().BoolVar(&layoutVertical, "vertical", false, "vertical pane layout")
	ssh.Flags().BoolVar(&layoutHorizontal, "horizontal", false, "horizontal pane layout")
	ssh.MarkFlagsMutuallyExclusive("vertical", "horizontal")

	// windows has no such thing as execve
	if runtime.GOOS == "windows" {
		ssh.Hidden = true
	}

	RootCmd.AddCommand(ssh)
}

func tmuxCommandFromTargets(targets target.Collection) []string {
	// sessions should be unique enough that running multiple sessions
	// does not collide
	// we take a random number from 0 to MaxInt encoded as base36
	session := fmt.Sprintf("iectl_%06s", strconv.FormatInt(int64(rand.Intn(int(^uint(0)>>1))), 36))

	// sshCmd with backstop
	// if ssh exits with non-zero exitcode, make sure the user is able to
	// read the actual error message - otherwize tmux just closes the pane.
	sshCmd := `sh -c 'ssh %s@%s || { code=$?; echo -e "\033[1;31m[iectl] SSH failed with code $code\033[0m"; echo -e "\033[0;33m[iectl] Press Enter to close this pane...\033[0m"; read; }'`

	targs := []string{
		// not really sure why the first argument has to be tmux - we are after all
		// Exec'ing the path of tmux, and not just tmux.....
		"tmux",
		"new-session", "-d", "-s", session,
		// make the detached session big enough for a handful of pane's
		"-x", "1200", "-y", "1200",
		fmt.Sprintf(sshCmd, user, targets[0].Hostname),
	}

	for _, t := range targets[1:] {
		targs = append(targs,
			";", "split-window", "-t", session,
			fmt.Sprintf(sshCmd, user, t.Hostname),
		)
	}

	// default best effort'ish
	layout := "tiled"

	if layoutVertical {
		layout = "even-vertical"
	}

	if layoutHorizontal {
		layout = "even-horizontal"
	}

	targs = append(targs,
		";", "select-layout", "-t", session, layout,
		";", "attach", "-t", session,
	)

	if synchronizePanes {
		targs = append(targs, ";", "setw", "-t", session, "synchronize-panes", "on",
			";", "display-popup", "echo 'Hello from iectl\nsynchronize-panes is on\n\nIf you want to turn it off:\n\nC-b :setw synchronize-panes off\nor add --synchronize-panes=false\n\nClose this popup with Escape or C-c'",
		)
	}

	return targs
}
