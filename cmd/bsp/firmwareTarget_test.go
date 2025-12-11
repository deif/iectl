package bsp

import (
	"context"
	"sync"
	"testing"

	"github.com/deif/iectl/auth"
	"github.com/deif/iectl/target"
)

// TODO: implement a mock webserver instead of using a real device....
// these tests will fail unless a physical device is available on the following
// hostname
const HOSTNAME = "iE250-05eb2f.local"

func TestFirmware(t *testing.T) {
	c, err := auth.Client(auth.WithInsecure, auth.WithCredentials(HOSTNAME, "admin", "admin"))
	if err != nil {
		t.Fatalf("cannot communicate: %s", err)

	}

	ft, err := newFirmwareTarget(
		target.Endpoint{Hostname: HOSTNAME, Client: c},
		"/home/fas/Downloads/ie250-mp-pcm21-v2.0.10.0-tc1.raucb",
	)

	if err != nil {
		t.Fatalf("cannot newFirmwareTarget: %s", err)
	}

	if ft.baseName != "ie250-mp-pcm21-v2.0.10.0-tc1.raucb" {
		t.Fatalf("Not correct name")
	}
	var sync sync.WaitGroup

	sync.Add(1)
	go func() {
		for v := range ft.LoadProgress {
			t.Logf("We have a loadprogress: %+v", v)
		}

		t.Log("LoadProgress closed")
		sync.Done()
	}()

	err = ft.LoadFirmware(context.Background(), 1)
	if err != nil {
		t.Fatalf("error to load firmware: %s", err)
	}

	sync.Add(1)
	go func() {
		for v := range ft.ApplyProgress {
			t.Logf("We have a applyprogress: %+v", v)
		}

		t.Log("ApplyProgress closed")
		sync.Done()
	}()

	err = ft.ApplyFirmware(context.Background(), 1)
	if err != nil {
		t.Fatalf("could not apply firmware: %s", err)
	}

	sync.Wait()
}
