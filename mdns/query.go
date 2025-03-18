package mdns

import (
	"fmt"
	"net"

	"github.com/miekg/dns"
)

// Query will emit dns messages to the mDNS multicast addresses
// it is up to the caller to listen for anwsers beforhand.
func Query(msg dns.Msg) error {
	payload, err := msg.Pack()
	if err != nil {
		return fmt.Errorf("unable to marshal dns message: %w", err)
	}

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

	// TODO: this does not do ipv6
	for _, iface := range interfaces {
		// Skip down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Create a UDP connection for sending
		conn, err := net.ListenMulticastUDP("udp4", &iface, udpAddr)
		if err != nil {
			return fmt.Errorf("error creating UDP connection: %w", err)

		}

		defer conn.Close()

		_, err = conn.WriteToUDP(payload, udpAddr)
		if err != nil {
			// fix this - what should we do?
			// thing is we should properly only
			// treat this as fatal if _all_ interfaces falied....
			// and then theres also ipv6....
		}
	}

	return nil
}
