package mdns

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type Browser struct {
	Question dns.Question
}

func (b *Browser) Run(ctx context.Context) (chan []*Target, error) {
	dnsChan, err := Listen(ctx)
	if err != nil {
		return nil, err
	}

	queryMsg := dns.Msg{}
	queryMsg.SetQuestion(b.Question.Name, b.Question.Qtype)
	go func() {
		timer := time.NewTimer(time.Second)
		round := 1
		for {
			err := Query(queryMsg)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}

				panic(err) // FIXME: not optimal
			}

			timer.Reset(expDuration(round))
			round++

			// Wait until the timer fires or the context is cancelled
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				continue
			}

		}
	}()

	targets := make([]*Target, 0)
	index := make(map[string]*Target)
	updates := make(chan []*Target)

	go func() {
		for {
			msg, ok := <-dnsChan
			if !ok {
				close(updates)
				break
			}
			for _, a := range msg.Answer {
				switch answer := a.(type) {
				case *dns.PTR:
					for _, v := range msg.Answer {
						if v.Header().Name == b.Question.Name {
							q := dns.Msg{}
							q.SetQuestion(dns.Fqdn(answer.Ptr), dns.TypeSRV)
							Query(q)
						}
					}
				case *dns.SRV:
					if !strings.HasSuffix(answer.Header().Name, b.Question.Name) {
						// We didnt ask for this
						continue
					}
					name := strings.TrimRight(answer.Target, ".")
					t, exists := index[name]
					if exists {
						continue
					}

					t = &Target{Hostname: name}
					index[name] = t
					targets = append(targets, t)

					updates <- targets
				}
			}
		}
	}()

	return updates, nil
}

// expDuration targets a sequence like
// 1s 2s 4s 8s 16s 32s 1m 1m 1m 1m 1m
func expDuration(i int) time.Duration {
	if i > 6 {
		return time.Minute
	}

	backoff := time.Second * (1 << i)
	if backoff > time.Minute {
		backoff = time.Minute
	}

	return backoff

}

type Target struct {
	Hostname string

	Marked bool
}

func (t *Target) Title() string {
	if !t.Marked {
		return t.Hostname
	}

	return fmt.Sprintf(">> %s", t.Hostname)
}

func (t *Target) Description() string {
	return fmt.Sprintf("https://%s/", t.Hostname)
}

func (t *Target) FilterValue() string {
	return t.Hostname
}
