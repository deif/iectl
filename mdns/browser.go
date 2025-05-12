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
		for {
			err := Query(queryMsg)
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
