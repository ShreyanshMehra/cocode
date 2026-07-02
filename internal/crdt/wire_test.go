package crdt

import (
	"encoding/json"
	"strings"
	"testing"
)

// These tests pin the JSON wire format so the Go server and the JavaScript
// client (web/crdt.js) stay interoperable: field names must match and a rune
// must serialise as its integer code point.

func TestOpJSONShape(t *testing.T) {
	op := Op{Type: OpInsert, ID: ID{Seq: 1, Site: 7}, Parent: Root, Char: 'h'}
	b, err := json.Marshal(op)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"type":"insert"`,
		`"id":{"seq":1,"site":7}`,
		`"parent":{"seq":0,"site":0}`,
		`"char":104`, // 'h' as an integer code point, matching JS
	} {
		if !strings.Contains(s, want) {
			t.Errorf("op JSON %s missing %s", s, want)
		}
	}
}

func TestApplyJSFormattedOps(t *testing.T) {
	// JSON exactly as web/crdt.js would emit for typing "hi" at the start.
	raw := []string{
		`{"type":"insert","id":{"seq":1,"site":42},"parent":{"seq":0,"site":0},"char":104}`,
		`{"type":"insert","id":{"seq":2,"site":42},"parent":{"seq":1,"site":42},"char":105}`,
	}
	d := New(0)
	for _, r := range raw {
		var op Op
		if err := json.Unmarshal([]byte(r), &op); err != nil {
			t.Fatal(err)
		}
		d.Apply(op)
	}
	if got := d.Value(); got != "hi" {
		t.Fatalf("applied JS-formatted ops -> %q, want %q", got, "hi")
	}
}
