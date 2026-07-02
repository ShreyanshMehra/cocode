// Command server runs the cocode collaborative editor: a WebSocket endpoint at
// /ws (join with ?room=<id>) plus the static web frontend.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/ShreyanshMehra/cocode/internal/hub"
)

func main() {
	h := hub.New()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.ServeWS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.Handle("/", http.FileServer(http.Dir("web")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	addr := ":" + port
	log.Printf("cocode listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
