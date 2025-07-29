package mdns

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/miekg/dns"
)

// Query will emit dns messages to the mDNS multicast addresses
// it is up to the caller to listen for anwsers beforhand.
func Query(msg dns.Msg) error {
	payload, err := msg.Pack()
	if err != nil {
		return fmt.Errorf("unable to marshal dns message: %w", err)
	}

	var (
		ipv4Err error
		ipv6Err error
	)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		ipv4Err = queryIPv4(payload)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		ipv6Err = queryIPv6(payload)
		wg.Done()
	}()

	wg.Wait()

	// only return error if everything fails ...
	// time will tell if: this was a good choice :)
	if ipv4Err != nil && ipv6Err != nil {
		return fmt.Errorf("both ipv4 and ipv6 failed on all interfaces\nipv4:%w\nipv6:%w", ipv4Err, ipv6Err)
	}

	return nil
}

func queryIPv6(payload []byte) error {
	multicastAddr := "[ff02::fb]:5353"
	udpAddr, err := net.ResolveUDPAddr("udp", multicastAddr)
	if err != nil {
		return fmt.Errorf("error resolving address: %w", err)

	}

	// Get all available network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("error getting interfaces: %w", err)

	}

	errs := make([]error, 0, len(interfaces))
	for _, iface := range interfaces {
		// Skip down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 {
			errs = append(errs, fmt.Errorf("%s: interface is not up", iface.Name))
			continue
		}

		if iface.Flags&net.FlagLoopback != 0 {
			errs = append(errs, fmt.Errorf("%s: loopback interface", iface.Name))
			continue
		}

		if iface.Flags&net.FlagPointToPoint != 0 {
			errs = append(errs, fmt.Errorf("%s: interface is point to point", iface.Name))
			continue
		}

		// Create a UDP connection for sending
		conn, err := net.ListenMulticastUDP("udp6", &iface, udpAddr)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: could not join udp6 multicast group (%T): %w", iface.Name, err, err))
			continue
		}

		defer conn.Close()

		_, err = conn.WriteToUDP(payload, udpAddr)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: failed to send udp4 dns query: %w", iface.Name, err))
			continue
		}
	}

	if len(errs) == len(interfaces) {
		return fmt.Errorf("all interfaces failed\n%w", errors.Join(errs...))
	}

	return nil
}

func queryIPv4(payload []byte) error {
	multicastAddr := "224.0.0.251:5353"
	udpAddr, err := net.ResolveUDPAddr("udp", multicastAddr)
	if err != nil {
		return fmt.Errorf("error resolving address: %w", err)

	}

	// Get all available network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("error getting interfaces: %w", err)

	}

	errs := make([]error, 0, len(interfaces))
	for _, iface := range interfaces {
		// Skip down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 {
			errs = append(errs, fmt.Errorf("%s: interface is not up", iface.Name))
			continue
		}

		if iface.Flags&net.FlagLoopback != 0 {
			errs = append(errs, fmt.Errorf("%s: loopback interface", iface.Name))
			continue
		}

		if iface.Flags&net.FlagPointToPoint != 0 {
			errs = append(errs, fmt.Errorf("%s: interface is point to point", iface.Name))
			continue
		}

		// Create a UDP connection for sending
		conn, err := net.ListenMulticastUDP("udp4", &iface, udpAddr)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: could not join udp4 multicast group (%T): %w", iface.Name, err, err))
			continue
		}

		defer conn.Close()

		_, err = conn.WriteToUDP(payload, udpAddr)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: failed to send udp4 dns query: %w", iface.Name, err))
			continue
		}
	}

	if len(errs) == len(interfaces) {
		return fmt.Errorf("all interfaces failed\n%w", errors.Join(errs...))
	}

	return nil
}
