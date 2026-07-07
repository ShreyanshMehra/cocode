// Package snapshot provides git-style versioning for collaborative documents.
//
// It deliberately reuses the ideas from a real Git object store: a document
// version's text is stored as a content-addressed *blob* (framed as
// "blob <len>\0<content>", hashed, and zlib-compressed under a fan-out
// directory), and versions form a parent chain like commits. Identical content
// is stored once (deduplicated by hash), and a version is an immutable record
// pointing at a blob plus its parent version.
//
// Unlike gitfromscratch (which mirrors Git exactly with SHA-1), this store uses
// SHA-256 — the framing and content-addressing concepts are identical.
package snapshot

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Version is an immutable snapshot record for one room's document.
type Version struct {
	ID       string `json:"id"`       // hash of this version record's identity
	Parent   string `json:"parent"`   // previous version id ("" for the first)
	Blob     string `json:"blob"`     // content-addressed hash of the document text
	Message  string `json:"message"`  // user-provided label
	UnixTime int64  `json:"unixTime"` // creation time
}

// Store persists blobs and per-room version logs under a base directory.
type Store struct {
	dir string
	mu  sync.Mutex
}

// New creates a Store rooted at dir, creating the objects directory.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "objects"), 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// hashBlob returns the content-addressed hash of framed content, matching git's
// object-identity scheme: sha256("blob <len>\0<content>").
func hashBlob(content []byte) (string, []byte) {
	framed := append([]byte(fmt.Sprintf("blob %d\x00", len(content))), content...)
	sum := sha256.Sum256(framed)
	return hex.EncodeToString(sum[:]), framed
}

func (s *Store) objectPath(hash string) string {
	return filepath.Join(s.dir, "objects", hash[:2], hash[2:])
}

// storeBlob writes framed content zlib-compressed; deduplicated by hash.
func (s *Store) storeBlob(content []byte) (string, error) {
	hash, framed := hashBlob(content)
	path := s.objectPath(hash)
	if _, err := os.Stat(path); err == nil {
		return hash, nil // already stored (dedup)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(framed); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return hash, os.WriteFile(path, buf.Bytes(), 0o644)
}

// readBlob loads and unframes a blob by hash.
func (s *Store) readBlob(hash string) (string, error) {
	f, err := os.Open(s.objectPath(hash))
	if err != nil {
		return "", fmt.Errorf("blob %s not found", hash)
	}
	defer f.Close()
	zr, err := zlib.NewReader(f)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	framed, err := io.ReadAll(zr)
	if err != nil {
		return "", err
	}
	nul := bytes.IndexByte(framed, 0)
	if nul < 0 {
		return "", fmt.Errorf("malformed blob %s", hash)
	}
	return string(framed[nul+1:]), nil
}

func (s *Store) versionsPath(roomID string) string {
	return filepath.Join(s.dir, "rooms", safe(roomID), "versions.jsonl")
}

// Save records a new version of room's document. Content is stored as a
// deduplicated blob; the version chains to the room's previous head.
func (s *Store) Save(roomID, content, message string) (Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	blob, err := s.storeBlob([]byte(content))
	if err != nil {
		return Version{}, err
	}
	versions, err := s.listLocked(roomID)
	if err != nil {
		return Version{}, err
	}
	parent := ""
	if n := len(versions); n > 0 {
		parent = versions[n-1].ID
	}
	v := Version{
		Parent:   parent,
		Blob:     blob,
		Message:  message,
		UnixTime: time.Now().Unix(),
	}
	v.ID = versionID(v)

	path := s.versionsPath(roomID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Version{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return Version{}, err
	}
	defer f.Close()
	line, _ := json.Marshal(v)
	if _, err := f.Write(append(line, '\n')); err != nil {
		return Version{}, err
	}
	return v, nil
}

// List returns a room's versions in creation order (oldest first).
func (s *Store) List(roomID string) ([]Version, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLocked(roomID)
}

func (s *Store) listLocked(roomID string) ([]Version, error) {
	data, err := os.ReadFile(s.versionsPath(roomID))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Version
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var v Version
		if err := json.Unmarshal(line, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// Content returns the document text for a given version id in a room.
func (s *Store) Content(roomID, id string) (string, error) {
	versions, err := s.List(roomID)
	if err != nil {
		return "", err
	}
	for _, v := range versions {
		if v.ID == id {
			return s.readBlob(v.Blob)
		}
	}
	return "", fmt.Errorf("version %s not found in room %s", id, roomID)
}

// versionID derives a stable id for a version from its identity fields.
func versionID(v Version) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d",
		v.Parent, v.Blob, v.Message, v.UnixTime)))
	return hex.EncodeToString(h[:])[:16]
}

// safe sanitises a room id for use as a directory name.
func safe(roomID string) string {
	if roomID == "" {
		return "default"
	}
	out := make([]rune, 0, len(roomID))
	for _, r := range roomID {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
