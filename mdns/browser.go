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
	Question dns.Msg
}

func (b *Browser) Run(ctx context.Context) (chan []*Target, error) {
	dnsChan, err := Listen(ctx)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			err := Query(b.Question)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}

				panic(err) // FIXME: not optimal
			}

			// FIXME: should backoff exponentially
			time.Sleep(time.Second * 1)
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
					q := dns.Msg{}
					q.SetQuestion(dns.Fqdn(answer.Ptr), dns.TypeSRV)
					Query(q)
				case *dns.SRV:
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

type Target struct {
	Hostname string
}

func (t Target) Title() string {
	return t.Hostname
}

func (t Target) Description() string {
	return fmt.Sprintf("https://%s/", t.Hostname)
}

func (t Target) FilterValue() string {
	return t.Hostname
}

func (t *Target) Update(d dns.RR) {
	// updating target
}

//log.Printf("%+v has update %+v", t, d)
