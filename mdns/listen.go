package mdns

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/miekg/dns"
)

type ErrListen struct {
	udp4 error
	udp6 error
}

func (e *ErrListen) Error() string {
	if e.udp4 != nil && e.udp6 != nil {
		return fmt.Sprintf("unable to listen on both ipv4 and ipv6: ipv4: %s, ipv6: %s", e.udp4, e.udp6)
	}

	if e.udp4 != nil {
		return fmt.Sprintf("unable to listen to on ipv4 multicast: %s", e.udp4)
	}

	if e.udp6 != nil {
		return fmt.Sprintf("unable to listen to on ipv6 multicast: %s", e.udp6)
	}

	return "No error.... go figure"
}

func Listen(ctx context.Context) (chan dns.Msg, error) {
	listeners := make([]*net.UDPConn, 0, 2)
	listenErr := ErrListen{}

	// try to listen for udp4 mDNS packets
	addr := net.UDPAddr{
		IP:   net.ParseIP("224.0.0.251"),
		Port: 5353,
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, &addr)
	if err != nil {
		listenErr.udp4 = err
	} else {
		listeners = append(listeners, conn)
	}

	// try to listen for udp6 mDNS packets
	addr = net.UDPAddr{
		IP:   net.ParseIP("ff02::fb"),
		Port: 5353,
	}

	conn, err = net.ListenMulticastUDP("udp6", nil, &addr)
	if err != nil {
		listenErr.udp6 = err
	} else {
		listeners = append(listeners, conn)
	}

	// if both failed, we cannot continue
	if len(listeners) == 0 {
		return nil, fmt.Errorf("no mDNS listeners made it: %w", &listenErr)
	}

	// we need the ability to wait for parsers to be shutdown
	// as simply closing the channel might cause a panic if a
	// parser tries to send on it
	var wg sync.WaitGroup

	c := make(chan dns.Msg)
	parse := func(c chan dns.Msg, conn *net.UDPConn) {
		buffer := make([]byte, 65536)
		for {
			n, _, err := conn.ReadFromUDP(buffer)
			if errors.Is(err, net.ErrClosed) {
				break
			}
			if err != nil {
				log.Printf("Error reading packet: %v", err)
				break

			}

			var msg dns.Msg
			if err := msg.Unpack(buffer[:n]); err != nil {
				log.Printf("Failed to parse mDNS packet: %v", err)
				continue
			}

			c <- msg
		}
		wg.Done()
	}

	for _, v := range listeners {
		wg.Add(1)
		go parse(c, v)
	}

	go func() {
		<-ctx.Done()
		for _, v := range listeners {
			err := v.Close()
			if err != nil {
				log.Printf("could not close mDNS listener: %s", err)
			}
		}

		// we must wait for parsers to shutdown
		// before closing their communication
		wg.Wait()

		close(c)
	}()

	return c, nil
}
