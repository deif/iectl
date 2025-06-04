package bsp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/hashicorp/yamux"
	"github.com/spf13/cobra"
)

var whatCmd = &cobra.Command{
	Use: "what",
	RunE: func(cmd *cobra.Command, args []string) error {
		u := url.URL{
			Scheme: "http",
			Host:   "iE250-05eb2f.local:3000",
			//Host: "localhost:3000",
			Path: "/",
		}
		//websocket.Dial()
		ctx := context.Background()

		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Upgrade", "not-a-websocket-but-a-special-deifsocket-socket")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusSwitchingProtocols {
			return fmt.Errorf("endpoint did not switch protocol")
		}

		rwc, ok := resp.Body.(io.ReadWriteCloser)
		if !ok {
			return fmt.Errorf("body is not ReadWriteCloser capable: %T", resp.Body)
		}

		session, err := yamux.Client(rwc, nil)
		if err != nil {
			panic(err)
		}

		err = serve(session)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		if err != nil {
			return err
		}
		//// Open a new stream
		//stream, err := session.Open()
		//if err != nil {
		//	panic(err)
		//}
		//
		//// Stream implements net.Conn
		//stream.Write([]byte("ping"))

		rwc.Close()
		return nil
	},
}

func init() {
	RootCmd.AddCommand(whatCmd)
}

func serve(l net.Listener) error {
	// var i int64
	r := chi.NewRouter()
	//r.Use(middleware.Logger)
	//r.Get("/", func(w http.ResponseWriter, r *http.Request) {
	//	fmt.Printf("Hurra!, vi har GET / igennem mux\n")
	//	fmt.Fprintf(w, "Hej server, i er %d", i)
	//	i++
	//})

	r.Put("/status", func(w http.ResponseWriter, r *http.Request) {
		msg, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
		if err != nil {
			http.Error(w, "Too large body", http.StatusBadRequest)
			return
		}

		fmt.Printf("We have progress: %s", msg)
	})

	r.Handle("/*", http.FileServer(http.Dir("/home/fas/Downloads")))

	return http.Serve(l, r)
}
