package bsp

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deif/iectl/target"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var (
	maxConcurrency int
)

func init() {
	firmwareCmd.Flags().IntVar(&maxConcurrency, "concurrency-limit", 10, "limit number of concurrent tasks")
	RootCmd.AddCommand(firmwareCmd)
}

var firmwareCmd = &cobra.Command{
	Use:   "install",
	Short: "Install new firmware on device",
	Args:  cobra.MatchAll(cobra.ExactArgs(1)),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		asJson, _ := cmd.Flags().GetBool("json")
		if asJson {
			return fmt.Errorf("install cant do --json")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// lets just check if we are able to open the file in question
		// we could also Stat the, but this dosnt take into account
		// if we are actually allowed to open the file
		fd, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("unable to open \"%q\": %w", args[0], err)
		}
		fd.Close()

		targets := target.FromContext(cmd.Context())
		firmwareTargets := make([]*firmwareTarget, 0, len(targets))
		for _, t := range targets {
			ft, err := newFirmwareTarget(
				target.Endpoint{Hostname: t.Hostname, Client: t.Client},
				args[0],
			)

			if err != nil {
				return fmt.Errorf("unable to prepare firmware task: %w", err)
			}

			firmwareTargets = append(firmwareTargets, ft)
		}

		// we now hold a bunch of firmwaretargets ready to proceed
		m, err := multiProgressModelWithTargets(firmwareTargets)
		if err != nil {
			return fmt.Errorf("unable to initialize ui: %w", err)
		}

		var uiGroup errgroup.Group
		ui := tea.NewProgram(m)
		uiGroup.Go(func() error {
			if _, err := ui.Run(); err != nil {
				return fmt.Errorf("ui failed: %w", err)
			}
			return nil
		})

		var loadGroup errgroup.Group
		loadGroup.SetLimit(maxConcurrency)
		for _, v := range firmwareTargets {
			loadGroup.Go(func() error {
				// feed status updates to ui
				go func() error {
					for {
						p, open := <-v.LoadProgress
						if !open {
							return nil
						}
						ui.Send(hostUpdate{p, v.Hostname})
					}
				}()

				// block here until the firmware is uploaded
				return v.LoadFirmware(context.Background(), 3)
			})

		}

		operationError := loadGroup.Wait()
		if operationError != nil {
			ui.Quit()
			err = uiGroup.Wait()

			operationError = errors.Join(operationError, err)

			return operationError
		}

		// well, we are here, all controllers have the file uploaded

		ui.Quit()
		err = uiGroup.Wait()
		return nil
	},
}
