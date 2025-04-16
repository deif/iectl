package bsp

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/deif/iectl/auth"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

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
		fd, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("unable to open \"%q\": %w", args[0], err)
		}
		defer fd.Close()

		info, err := fd.Stat()
		if err != nil {
			return fmt.Errorf("unable to stat \"%q\": %w", args[0], err)
		}

		client := auth.FromContext(cmd.Context())
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/firmware/file",
		}

		r, w, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("unable to create pipe: %w", err)
		}

		mp := multipart.NewWriter(w)
		multipartWriter, err := mp.CreateFormFile("file", filepath.Base(args[0]))

		if err != nil {
			return fmt.Errorf("unable to create multipart body: %s", err)
		}

		// Start Bubble Tea
		var p userInterface
		var rateLimiter *rate.Limiter
		interactive, _ := cmd.Flags().GetBool("interactive")
		if interactive {
			m := progressTUI{
				progress: progress.New(progress.WithDefaultGradient()),
				status:   "Connecting",
			}

			p = tea.NewProgram(m)
			rateLimiter = rate.NewLimiter(5, 1)

		} else {
			p = &boringProgress{Task: "Uploading"}
			rateLimiter = rate.NewLimiter(1, 1)
		}

		pWrite := io.MultiWriter(multipartWriter,
			&progressWriter{
				app:     p,
				limiter: rateLimiter,
				total:   int(info.Size()),
			})

		fmt.Printf("Uploading firmware\n")

		var group errgroup.Group
		group.Go(func() error {
			_, err := io.Copy(pWrite, fd)
			if err != nil {
				return fmt.Errorf("unable to copy file %w", err)
			}

			err = mp.Close()
			if err != nil {
				return fmt.Errorf("unable to close multipart writer: %w", err)
			}

			err = w.Close()
			if err != nil {
				return fmt.Errorf("unable to close pipe: %w", err)
			}

			return nil
		})

		group.Go(func() error {
			req, err := http.NewRequest("POST", u.String(), r)
			if err != nil {
				return fmt.Errorf("unable to create http request: %w", err)
			}
			req.Header.Set("Content-Type", mp.FormDataContentType())

			// We happen to know that the multipart writers various headers and stuff
			// is usually 232 bytes long - given that the content-type and fieldname dosnt change
			// - all we need is add the actual file size and size of the filename
			// what could go wrong ? - off the top of my head, the only solid approach would
			// be to send the request to /dev/null first and count the bytes - but lets see
			// if we can get by using this constant...
			req.ContentLength = 232 + info.Size() + int64(len(filepath.Base(args[0])))

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("unable to http post: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
			}

			p.Send(progressMsg{ratio: 1, status: "Finished uploading"})

			return nil
		})

		group.Go(func() error {
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("unable to start gui: %w", err)
			}

			return nil
		})

		err = group.Wait()
		if err != nil {
			return fmt.Errorf("upload filed: %w", err)
		}

		// but we are not finished - we have the file uploaded, now we must
		// ask the device to apply the update

		u.Path = "/bsp/firmware/upgrade"
		req, err := http.NewRequest("PUT", u.String(), nil)
		if err != nil {
			return fmt.Errorf("unable to create http put request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("unable to call http endpoint: %w", err)
		}

		if resp.StatusCode != http.StatusAccepted {
			return fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
		}

		fmt.Printf("\n\nUpgrading firmware\n")

		// Lets do another progressbar
		var interval time.Duration
		if interactive {
			m := progressTUI{
				progress: progress.New(progress.WithDefaultGradient()),
				status:   "Connecting",
			}
			p = tea.NewProgram(m)
			interval = time.Second / 4
		} else {
			p = &boringProgress{Task: "Upgrading"}
			interval = time.Second
		}
		// we need to deal with errors from both the progress routine
		// and the app.

		group.Go(func() error {
			var state = struct {
				Lines []struct {
					Progress int
					Text     string
				}
			}{}
			for {
				time.Sleep(interval)
				resp, err := client.Get(u.String())
				if err != nil {
					p.Send(progressMsg{ratio: 0, status: fmt.Sprintf("ERR: %s", err)})
					continue
				}

				switch resp.StatusCode {
				case http.StatusOK:
				case http.StatusAccepted:
				case http.StatusCreated:
					p.Quit()
					fmt.Printf("\n\nFirmware upgraded\n")
					return nil
				case http.StatusNotFound:
					fmt.Printf("\n\nNo firmware found\n")
					p.Quit()
					return nil
				case http.StatusInternalServerError:
					p.Quit()
					return fmt.Errorf("Failed to install image")
				default:
					p.Quit()
					return fmt.Errorf("Unexpected status code %d", resp.StatusCode)
				}

				dec := json.NewDecoder(resp.Body)
				err = dec.Decode(&state)
				if err != nil {
					p.Send(progressMsg{ratio: 0, status: fmt.Sprintf("ERR: %s", err)})
					resp.Body.Close()
					continue
				}

				resp.Body.Close()

				p.Send(progressMsg{
					ratio:  float64(state.Lines[0].Progress) / 100,
					status: state.Lines[0].Text,
				})
			}
		})
		group.Go(func() error {
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("unable to start gui: %w", err)
			}
			return nil
		})

		// wait for tui and status loop to exit
		err = group.Wait()
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(firmwareCmd)
}

type boringProgress struct {
	Task string
}

func (b *boringProgress) Send(m tea.Msg) {
	switch v := m.(type) {
	case progressMsg:
		fmt.Printf("%s: %d%% - %s\n", b.Task, int(v.ratio*100), v.status)
	default:
		fmt.Printf("%s: %s\n", b.Task, m)
	}
}

func (b *boringProgress) Run() (tea.Model, error) {
	return nil, nil
}

func (b *boringProgress) Quit() {}

type userInterface interface {
	Send(tea.Msg)
	Run() (tea.Model, error)
	Quit()
}

type progressWriter struct {
	app     userInterface
	written int
	total   int
	limiter *rate.Limiter
}

func (p *progressWriter) Write(in []byte) (int, error) {
	p.written += len(in)
	if p.limiter.Allow() {
		p.app.Send(
			progressMsg{
				ratio:  float64(p.written) / float64(p.total),
				status: fmt.Sprintf("%s of %s", humanize.Bytes(uint64(p.written)), humanize.Bytes(uint64(p.total))),
			})
	}
	return len(in), nil
}
