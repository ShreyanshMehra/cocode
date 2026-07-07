package snapshot

import (
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSaveAndListChain(t *testing.T) {
	s := newStore(t)
	v1, err := s.Save("room1", "hello", "first")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := s.Save("room1", "hello world", "second")
	if err != nil {
		t.Fatal(err)
	}
	if v1.Parent != "" {
		t.Errorf("first version parent = %q, want empty", v1.Parent)
	}
	if v2.Parent != v1.ID {
		t.Errorf("second parent = %q, want %q", v2.Parent, v1.ID)
	}
	list, err := s.List("room1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != v1.ID || list[1].ID != v2.ID {
		t.Errorf("unexpected version list: %+v", list)
	}
}

func TestContentRoundTrip(t *testing.T) {
	s := newStore(t)
	v, _ := s.Save("r", "the quick brown fox\n", "msg")
	got, err := s.Content("r", v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != "the quick brown fox\n" {
		t.Errorf("content = %q, want original", got)
	}
}

func TestBlobDeduplication(t *testing.T) {
	s := newStore(t)
	// Same content in two rooms must hash to the same blob.
	v1, _ := s.Save("a", "identical", "x")
	v2, _ := s.Save("b", "identical", "y")
	if v1.Blob != v2.Blob {
		t.Errorf("identical content produced different blobs: %s vs %s", v1.Blob, v2.Blob)
	}
}

func TestContentAddressingDiffersByContent(t *testing.T) {
	s := newStore(t)
	v1, _ := s.Save("r", "aaa", "1")
	v2, _ := s.Save("r", "bbb", "2")
	if v1.Blob == v2.Blob {
		t.Error("different content should produce different blob hashes")
	}
}

func TestListEmptyRoom(t *testing.T) {
	s := newStore(t)
	list, err := s.List("never-used")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("expected no versions, got %d", len(list))
	}
}

func TestContentUnknownVersionErrors(t *testing.T) {
	s := newStore(t)
	if _, err := s.Content("r", "deadbeef"); err == nil {
		t.Error("expected error for unknown version id")
	}
}

func TestPersistsAcrossStoreInstances(t *testing.T) {
	dir := t.TempDir()
	s1, _ := New(dir)
	v, _ := s1.Save("r", "persisted", "m")

	s2, _ := New(dir) // fresh instance, same dir
	got, err := s2.Content("r", v.ID)
	if err != nil || got != "persisted" {
		t.Errorf("reload failed: got %q err %v", got, err)
	}
}
