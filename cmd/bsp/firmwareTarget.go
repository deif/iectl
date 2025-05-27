package bsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/deif/iectl/target"
	"github.com/dustin/go-humanize"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

type progressWriter2 struct {
	sync.RWMutex
	channel chan progressMsg

	written         int
	lastWritten     int
	lastWrittenTime time.Time
	total           int64

	limiter          *rate.Limiter
	stallTimer       *time.Timer
	stopStallRoutine chan struct{}
}

func (p *progressWriter2) Initialize() {
	p.stallTimer = time.NewTimer(time.Second)
	p.stopStallRoutine = make(chan struct{})

	go func() {
		for {
			select {
			case <-p.stopStallRoutine:
				return
			case <-p.stallTimer.C:
			}

			if p.limiter.Allow() {
				p.RLock()
				msg := progressMsg{
					ratio: float64(p.written) / float64(p.total),
					status: fmt.Sprintf("%s of %s (stalled, timeout in %s)",
						humanize.Bytes(uint64(p.written)),
						humanize.Bytes(uint64(p.total)),
						(60*time.Second - time.Now().Sub(p.lastWrittenTime)).Truncate(time.Second),
					),
				}
				p.RUnlock()

				p.channel <- msg
			}

			p.stallTimer.Reset(time.Second)
		}

	}()
}

func (p *progressWriter2) Write(in []byte) (int, error) {
	p.Lock()
	defer p.Unlock()

	p.lastWrittenTime = time.Now()
	p.written += len(in)
	p.stallTimer.Reset(time.Second)

	if p.limiter.Allow() {
		rate := (p.written - p.lastWritten) * int(p.limiter.Limit())
		p.channel <- progressMsg{
			ratio:  float64(p.written) / float64(p.total),
			status: fmt.Sprintf("%s of %s (%s/sec)", humanize.Bytes(uint64(p.written)), humanize.Bytes(uint64(p.total)), humanize.Bytes(uint64(rate))),
		}

		p.lastWritten = p.written
	}
	return len(in), nil
}

func (p *progressWriter2) Close() error {
	p.stallTimer.Stop()
	close(p.channel)
	close(p.stopStallRoutine)
	return nil
}

type firmwareTarget struct {
	target.Endpoint

	fd       *os.File
	info     os.FileInfo
	baseName string

	LoadProgress  chan progressMsg
	ApplyProgress chan progressMsg
}

func newFirmwareTarget(t target.Endpoint, firmwareBlobPath string) (*firmwareTarget, error) {
	ft := firmwareTarget{
		Endpoint:      t,
		LoadProgress:  make(chan progressMsg),
		ApplyProgress: make(chan progressMsg),
	}

	var err error
	ft.fd, err = os.Open(firmwareBlobPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open \"%q\": %w", firmwareBlobPath, err)
	}

	ft.info, err = ft.fd.Stat()
	if err != nil {
		return nil, fmt.Errorf("unable to stat \"%q\": %w", firmwareBlobPath, err)
	}

	ft.baseName = filepath.Base(firmwareBlobPath)

	return &ft, nil
}

// LoadFirmware updates the firmware in question to the target
// the caller is responsible for emptying LoadProgress or
// the process will lock up
func (f *firmwareTarget) LoadFirmware(ctx context.Context, rateLimit rate.Limit) error {
	r, w, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("unable to create pipe: %w", err)
	}

	mp := multipart.NewWriter(w)
	partWriter, err := mp.CreateFormFile("file", f.baseName)

	if err != nil {
		return fmt.Errorf("unable to create multipart body: %s", err)
	}

	progress := &progressWriter2{
		channel: f.LoadProgress,
		limiter: rate.NewLimiter(rateLimit, 1),
		total:   f.info.Size(),
	}
	progress.Initialize()

	pWrite := io.MultiWriter(partWriter,
		progress)

	var eGroup errgroup.Group
	eGroup.Go(func() error {
		_, copyError := io.Copy(pWrite, f.fd)

		// close multipart writer
		mp.Close()

		// close pipe
		w.Close()

		// close file
		f.fd.Close()

		if copyError != nil {
			f.LoadProgress <- progressMsg{err: fmt.Sprintf("Failed: %s", copyError), ratio: 0.0}
		} else {
			f.LoadProgress <- progressMsg{status: "Successfully uploaded file", ratio: 1.0}
		}

		// close the progress writer
		progress.Close()

		return copyError
	})

	u := url.URL{
		Scheme: "https",
		Host:   f.Hostname,
		Path:   "/bsp/firmware/file",
	}

	f.LoadProgress <- progressMsg{ratio: 0, status: "Connecting..."}

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), r)
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
	req.ContentLength = 232 + f.info.Size() + int64(len(f.baseName))

	resp, err := f.Client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to http post: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	err = eGroup.Wait()
	if err != nil {
		return err
	}

	return nil
}

func (f *firmwareTarget) ApplyFirmware(ctx context.Context, rateLimit rate.Limit) error {
	u := url.URL{
		Scheme: "https",
		Host:   f.Hostname,
		Path:   "/bsp/firmware/upgrade",
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", u.String(), nil)
	if err != nil {
		return fmt.Errorf("unable to create http put request: %w", err)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return fmt.Errorf("unable to call http endpoint: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}

	var state = struct {
		Lines []struct {
			Progress int
			Text     string
		}
	}{}

	for {
		// wait here for interval or fail if our
		// deadline is exceeded or the context is cancelled.
		// (we are converting Hz to time.Duration - for consistency with LoadFirmware)
		t := time.NewTimer(time.Duration(float64(time.Second) / float64(rateLimit)))
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}

		// TODO: can this req togeather with its context be resued as is?
		// - not that it will really matter a lot....
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return fmt.Errorf("unable to create http request: %w", err)
		}

		resp, err = f.Client.Do(req)
		if err != nil {
			f.ApplyProgress <- progressMsg{ratio: 0, status: fmt.Sprintf("ERR: %s", err)}
			continue
		}
		switch resp.StatusCode {
		case http.StatusOK:
		case http.StatusAccepted:
		case http.StatusCreated:
			resp.Body.Close()
			f.ApplyProgress <- progressMsg{ratio: 1.0, status: "Successfully installed firmware."}
			close(f.ApplyProgress)

			return nil
		case http.StatusNotFound:
			return fmt.Errorf("device answers: no firmware found")
		case http.StatusInternalServerError:
			return fmt.Errorf("failed to install image: internal server error")
		default:
			return fmt.Errorf("unexpected status code %d", resp.StatusCode)
		}

		// StatusAccepted and StatusCreated have progress in their bodies
		dec := json.NewDecoder(resp.Body)
		err = dec.Decode(&state)
		if err != nil {
			// this is kind of a weird error state, the webserver said a successive
			// statuscode, but we cannot figure out how to parse the progress message
			// - so i think we should continue without stopping everything.
			f.ApplyProgress <- progressMsg{ratio: 0, status: fmt.Sprintf("ERR: %s", err)}
			resp.Body.Close()
			continue
		}

		resp.Body.Close()

		f.ApplyProgress <- progressMsg{
			ratio:  float64(state.Lines[0].Progress) / 100,
			status: state.Lines[0].Text,
		}
	}
}
