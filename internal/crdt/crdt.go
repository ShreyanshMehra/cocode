// Package crdt implements a sequence CRDT for collaborative text editing using
// a causal tree (a.k.a. timestamped insertion tree), which is equivalent to
// RGA (Replicated Growable Array).
//
// The idea: every inserted character is an "atom" with a globally-unique ID and
// a parent — the ID of the atom it was inserted immediately after. All atoms
// form a tree rooted at a virtual root. The visible document is the pre-order
// DFS of that tree, where the children of each node are ordered by DESCENDING
// ID. Deletions are tombstones (the atom stays in the tree, marked deleted).
//
// Why this converges: applying an atom only attaches it to its parent, and the
// sibling ordering is a pure function of IDs, so every replica builds the exact
// same tree regardless of the order in which it receives operations. The DFS is
// deterministic, therefore every replica renders the same string. Operations are
// also idempotent (re-applying an atom is a no-op) and commutative.
package crdt

import (
	"sort"
	"strings"
)

// ID uniquely identifies an atom. The zero value is the virtual Root. Ordering
// is Lamport-like: higher Seq wins, ties broken by Site, so "more recent" edits
// sort first among concurrent siblings.
type ID struct {
	Seq  uint64
	Site uint64
}

// Root is the virtual parent of the first characters in the document.
var Root = ID{}

// less reports whether a sorts before b in ascending order.
func (a ID) less(b ID) bool {
	if a.Seq != b.Seq {
		return a.Seq < b.Seq
	}
	return a.Site < b.Site
}

// OpType is the kind of an operation.
type OpType string

const (
	OpInsert OpType = "insert"
	OpDelete OpType = "delete"
)

// Op is a replicated operation. Inserts carry Parent and Char; deletes carry
// only the target ID. Ops are the unit exchanged between replicas.
type Op struct {
	Type   OpType `json:"type"`
	ID     ID     `json:"id"`
	Parent ID     `json:"parent,omitempty"`
	Char   rune   `json:"char,omitempty"`
}

type atom struct {
	id       ID
	parent   ID
	char     rune
	deleted  bool
	children []ID // kept sorted DESCENDING by ID
}

// Doc is a single replica of a collaborative document.
type Doc struct {
	site    uint64
	clock   uint64
	atoms   map[ID]*atom
	pending map[ID][]Op // inserts waiting for an unknown parent (out-of-order)
}

// New creates an empty document replica identified by a unique site id.
func New(site uint64) *Doc {
	d := &Doc{
		site:    site,
		atoms:   map[ID]*atom{},
		pending: map[ID][]Op{},
	}
	d.atoms[Root] = &atom{id: Root}
	return d
}

// nextID returns a fresh, unique, Lamport-incremented id for a local op.
func (d *Doc) nextID() ID {
	d.clock++
	return ID{Seq: d.clock, Site: d.site}
}

// LocalInsert inserts ch at visible index (0..len) and returns the op to
// broadcast. Index 0 inserts at the very start; index == length appends.
func (d *Doc) LocalInsert(index int, ch rune) Op {
	parent := d.parentForIndex(index)
	op := Op{Type: OpInsert, ID: d.nextID(), Parent: parent, Char: ch}
	d.Apply(op)
	return op
}

// LocalDelete tombstones the atom at visible index and returns the op. The
// second result is false if the index is out of range.
func (d *Doc) LocalDelete(index int) (Op, bool) {
	vis := d.visibleIDs()
	if index < 0 || index >= len(vis) {
		return Op{}, false
	}
	op := Op{Type: OpDelete, ID: vis[index]}
	d.Apply(op)
	return op, true
}

// Apply integrates a (local or remote) op. It is idempotent and commutative.
// Inserts whose parent is not yet known are buffered and flushed once the
// parent arrives, so out-of-order delivery is handled safely.
func (d *Doc) Apply(op Op) {
	switch op.Type {
	case OpInsert:
		d.applyInsert(op)
	case OpDelete:
		if a := d.atoms[op.ID]; a != nil {
			a.deleted = true
		}
	}
}

func (d *Doc) applyInsert(op Op) {
	if _, exists := d.atoms[op.ID]; exists {
		return // idempotent
	}
	parent := d.atoms[op.Parent]
	if parent == nil {
		// Parent unknown yet: buffer until it arrives (out-of-order delivery).
		d.pending[op.Parent] = append(d.pending[op.Parent], op)
		return
	}
	// Keep the local Lamport clock ahead of anything we have seen.
	if op.ID.Seq > d.clock {
		d.clock = op.ID.Seq
	}
	a := &atom{id: op.ID, parent: op.Parent, char: op.Char}
	d.atoms[op.ID] = a
	parent.children = insertDesc(parent.children, op.ID)

	// Flush any operations that were waiting on this atom as their parent.
	if waiting, ok := d.pending[op.ID]; ok {
		delete(d.pending, op.ID)
		for _, w := range waiting {
			d.applyInsert(w)
		}
	}
}

// insertDesc inserts id into a slice kept in descending order.
func insertDesc(ids []ID, id ID) []ID {
	i := sort.Search(len(ids), func(i int) bool { return ids[i].less(id) })
	ids = append(ids, ID{})
	copy(ids[i+1:], ids[i:])
	ids[i] = id
	return ids
}

// Value renders the current visible document string.
func (d *Doc) Value() string {
	var sb strings.Builder
	d.walk(Root, func(a *atom) {
		if a.id != Root && !a.deleted {
			sb.WriteRune(a.char)
		}
	})
	return sb.String()
}

// visibleIDs returns the IDs of visible (non-deleted) atoms in document order.
func (d *Doc) visibleIDs() []ID {
	var ids []ID
	d.walk(Root, func(a *atom) {
		if a.id != Root && !a.deleted {
			ids = append(ids, a.id)
		}
	})
	return ids
}

// walk performs a pre-order DFS visiting each atom, children in descending ID.
func (d *Doc) walk(id ID, visit func(*atom)) {
	a := d.atoms[id]
	if a == nil {
		return
	}
	visit(a)
	for _, c := range a.children {
		d.walk(c, visit)
	}
}

// parentForIndex maps a visible insert index to the id to insert after.
func (d *Doc) parentForIndex(index int) ID {
	if index <= 0 {
		return Root
	}
	vis := d.visibleIDs()
	if index-1 < len(vis) {
		return vis[index-1]
	}
	if len(vis) > 0 {
		return vis[len(vis)-1]
	}
	return Root
}

// Len returns the number of visible characters.
func (d *Doc) Len() int { return len(d.visibleIDs()) }
