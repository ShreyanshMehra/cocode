// Command server runs the cocode collaborative editor:
//   - GET  /ws?room=<id>        WebSocket for real-time collaboration
//   - POST /api/snapshot        save the current room text as a version
//   - GET  /api/versions        list a room's saved versions
//   - GET  /api/version         fetch one version's content
//   - GET  /api/diff            unified line diff between two versions
//   - GET  /api/lang            detect a language from filename/content
//   - static web frontend at /
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ShreyanshMehra/cocode/internal/hub"
	"github.com/ShreyanshMehra/cocode/internal/lang"
	"github.com/ShreyanshMehra/cocode/internal/snapshot"
	"github.com/ShreyanshMehra/cocode/internal/versiondiff"
)

type server struct {
	hub   *hub.Hub
	store *snapshot.Store
}

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "cocode-data")
	}
	store, err := snapshot.New(dataDir)
	if err != nil {
		log.Fatalf("snapshot store: %v", err)
	}
	s := &server{hub: hub.New(), store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.hub.ServeWS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/snapshot", s.handleSnapshot)
	mux.HandleFunc("/api/versions", s.handleVersions)
	mux.HandleFunc("/api/version", s.handleVersion)
	mux.HandleFunc("/api/diff", s.handleDiff)
	mux.HandleFunc("/api/lang", s.handleLang)
	mux.Handle("/", http.FileServer(http.Dir("web")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	addr := ":" + port
	log.Printf("cocode listening on %s (data: %s)", addr, dataDir)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func roomParam(r *http.Request) string {
	room := r.URL.Query().Get("room")
	if room == "" {
		room = "default"
	}
	return room
}

// handleSnapshot saves the room's current canonical text as a new version.
func (s *server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	room := roomParam(r)
	content, ok := s.hub.RoomValue(room)
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	v, err := s.store.Save(room, content, r.URL.Query().Get("message"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// handleVersions lists a room's saved versions (oldest first).
func (s *server) handleVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := s.store.List(roomParam(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

// handleVersion returns the text content of a single version.
func (s *server) handleVersion(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	content, err := s.store.Content(roomParam(r), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "content": content})
}

// handleDiff returns a unified line diff between two versions (a -> b).
func (s *server) handleDiff(w http.ResponseWriter, r *http.Request) {
	room := roomParam(r)
	aContent, err := s.store.Content(room, r.URL.Query().Get("a"))
	if err != nil {
		http.Error(w, "version a: "+err.Error(), http.StatusNotFound)
		return
	}
	bContent, err := s.store.Content(room, r.URL.Query().Get("b"))
	if err != nil {
		http.Error(w, "version b: "+err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"diff": versiondiff.Unified(aContent, bContent),
	})
}

// handleLang detects a language from a filename and/or content sample.
func (s *server) handleLang(w http.ResponseWriter, r *http.Request) {
	l := lang.Detect(r.URL.Query().Get("filename"), r.URL.Query().Get("content"))
	writeJSON(w, http.StatusOK, l)
}
