package bsp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hashicorp/yamux"
	"github.com/spf13/cobra"
)

var whatCmd = &cobra.Command{
	Use: "what",
	RunE: func(cmd *cobra.Command, args []string) error {
		u := url.URL{
			Scheme: "ws",
			Host:   "localhost:3000",
			Path:   "/",
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		c, _, err := websocket.Dial(ctx, u.String(), nil)
		if err != nil {
			return err
		}
		defer c.CloseNow()

		session, err := yamux.Client(websocket.NetConn(ctx, c, websocket.MessageBinary), nil)
		if err != nil {
			panic(err)
		}

		serve(session)
		//// Open a new stream
		//stream, err := session.Open()
		//if err != nil {
		//	panic(err)
		//}
		//
		//// Stream implements net.Conn
		//stream.Write([]byte("ping"))

		c.Close(websocket.StatusNormalClosure, "")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(whatCmd)
}

func serve(l net.Listener) error {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Hurra!, vi har GET / igennem mux\n")
		fmt.Fprintf(w, "Hej server")
	})

	return http.Serve(l, r)
}
