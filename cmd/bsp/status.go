package bsp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/deif/iectl/auth"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "General system status",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := auth.FromContext(cmd.Context())
		host, _ := cmd.Flags().GetString("hostname")
		u := url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/bsp/system/status",
		}

		resp, err := client.Get(u.String())
		if err != nil {
			return fmt.Errorf("unable to http get: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected statuscode: %d", resp.StatusCode)
		}

		asJson, _ := cmd.Flags().GetBool("json")
		if asJson {
			_, err = io.Copy(os.Stdout, resp.Body)
			if err != nil {
				return fmt.Errorf("unable to copy to stdout: %w", err)
			}
			return nil
		}

		dec := json.NewDecoder(resp.Body)
		d := &Device{}
		err = dec.Decode(d)
		if err != nil {
			return fmt.Errorf("unable to unmarshal status message: %w", err)
		}

		printDeviceInfo(d)

		return nil
	},
}

type Interface struct {
	Description string `json:"description"`
	Ifname      string `json:"ifname"`
	Kind        string `json:"kind"`
	Status      struct {
		LinkState  string `json:"link_state"`
		MacAddress string `json:"mac_address"`
		IPv4       *struct {
			IP           string `json:"ip"`
			PrefixLength int    `json:"prefix_length"`
		} `json:"ipv4,omitempty"`
		IPv6 *struct {
			IP           string `json:"ip"`
			PrefixLength int    `json:"prefix_length"`
		} `json:"ipv6,omitempty"`
	} `json:"status"`
}

type MountPoint struct {
	MountPoint string `json:"mountPoint"`
	Size       uint64 `json:"size"`
	Used       uint64 `json:"used"`
}

type Software struct {
	A      string `json:"A"`
	B      string `json:"B"`
	Active string `json:"active"`
}

type Device struct {
	Hostname    string       `json:"hostname"`
	Interfaces  []Interface  `json:"interfaces"`
	Mountpoints []MountPoint `json:"mountpoints"`
	Serial      string       `json:"serialnumber"`
	Software    Software     `json:"software"`
}

func init() {
	RootCmd.AddCommand(statusCmd)
}

func printDeviceInfo(d *Device) {
	fmt.Println("========================")
	fmt.Printf("Device Hostname: %s\n", d.Hostname)
	fmt.Printf("Serial Number: %s\n", d.Serial)
	fmt.Println("========================")
	fmt.Println("Network Interfaces:")
	for _, iface := range d.Interfaces {
		fmt.Printf("- %s (%s)\n", iface.Ifname, iface.Kind)
		fmt.Printf("  Description: %s\n", iface.Description)
		fmt.Printf("  MAC Address: %s\n", iface.Status.MacAddress)
		fmt.Printf("  Link State: %s\n", iface.Status.LinkState)
		if iface.Status.IPv4 != nil {
			fmt.Printf("  IPv4: %s/%d\n", iface.Status.IPv4.IP, iface.Status.IPv4.PrefixLength)
		}
		if iface.Status.IPv6 != nil {
			fmt.Printf("  IPv6: %s/%d\n", iface.Status.IPv6.IP, iface.Status.IPv6.PrefixLength)
		}
		fmt.Println(strings.Repeat("-", 40))
	}

	fmt.Println("Storage Mountpoints:")
	for _, mp := range d.Mountpoints {
		fmt.Printf("- Mount Point: %s\n", mp.MountPoint)
		fmt.Printf("  Size: %s\n", humanize.Bytes(mp.Size))
		fmt.Printf("  Used: %s\n", humanize.Bytes(mp.Used))
		fmt.Println(strings.Repeat("-", 40))
	}

	fmt.Println("Software Versions:")
	fmt.Printf("- Version A: %s\n", d.Software.A)
	fmt.Printf("- Version B: %s\n", d.Software.B)
	fmt.Printf("- Active Version: %s", d.Software.Active)
	if d.Software.Active == "A" {
		fmt.Printf(" (%s)", d.Software.A)
	}
	if d.Software.Active == "B" {
		fmt.Printf(" (%s)", d.Software.B)
	}
	fmt.Println()
	fmt.Println("========================")
}
