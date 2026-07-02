package crdt

import (
	"math/rand"
	"testing"
)

// typeString inserts each rune of s starting at index `at`, sequentially.
func typeString(d *Doc, at int, s string) []Op {
	var ops []Op
	for i, ch := range s {
		ops = append(ops, d.LocalInsert(at+i, ch))
	}
	return ops
}

func TestLocalTyping(t *testing.T) {
	d := New(1)
	typeString(d, 0, "hello")
	if got := d.Value(); got != "hello" {
		t.Fatalf("Value = %q, want %q", got, "hello")
	}
	if d.Len() != 5 {
		t.Errorf("Len = %d, want 5", d.Len())
	}
}

func TestInsertInMiddle(t *testing.T) {
	d := New(1)
	typeString(d, 0, "helo")
	d.LocalInsert(3, 'l') // "hel" + "l" + "o" -> "hello"
	if got := d.Value(); got != "hello" {
		t.Fatalf("Value = %q, want %q", got, "hello")
	}
}

func TestDeleteTombstones(t *testing.T) {
	d := New(1)
	typeString(d, 0, "hello")
	// delete the two 'l's (indices shift as we delete left-to-right)
	d.LocalDelete(2) // remove first 'l' -> "helo"
	d.LocalDelete(2) // remove second 'l' -> "heo"
	if got := d.Value(); got != "heo" {
		t.Fatalf("Value = %q, want %q", got, "heo")
	}
	if _, ok := d.LocalDelete(99); ok {
		t.Error("delete out of range should return false")
	}
}

func TestIdempotentApply(t *testing.T) {
	d := New(1)
	op := d.LocalInsert(0, 'x')
	before := d.Value()
	d.Apply(op) // re-apply the same op
	d.Apply(op)
	if d.Value() != before {
		t.Errorf("re-applying an op changed the doc: %q -> %q", before, d.Value())
	}
}

// applyAll applies ops to d (used to simulate receiving remote ops).
func applyAll(d *Doc, ops []Op) {
	for _, op := range ops {
		d.Apply(op)
	}
}

func TestTwoReplicasConverge(t *testing.T) {
	a := New(1)
	b := New(2)

	// Both start from the same base "ab" (a types, b receives).
	base := typeString(a, 0, "ab")
	applyAll(b, base)
	if a.Value() != b.Value() {
		t.Fatalf("base diverged: %q vs %q", a.Value(), b.Value())
	}

	// Concurrent edits: a inserts 'X' at start, b inserts 'Y' at start.
	opA := a.LocalInsert(0, 'X')
	opB := b.LocalInsert(0, 'Y')

	// Exchange.
	b.Apply(opA)
	a.Apply(opB)

	if a.Value() != b.Value() {
		t.Fatalf("replicas diverged after concurrent insert: %q vs %q",
			a.Value(), b.Value())
	}
	// Deterministic tie-break: site 2 (higher) sorts first among equal Seq.
	if a.Value() != "YXab" {
		t.Errorf("Value = %q, want %q (deterministic order)", a.Value(), "YXab")
	}
}

func TestOutOfOrderDelivery(t *testing.T) {
	src := New(1)
	ops := typeString(src, 0, "abc") // op0='a'@root, op1='b'@a, op2='c'@b

	dst := New(2)
	// Deliver in REVERSE order: child before parent. The pending buffer must
	// hold them until parents arrive, then converge.
	for i := len(ops) - 1; i >= 0; i-- {
		dst.Apply(ops[i])
	}
	if dst.Value() != "abc" {
		t.Fatalf("out-of-order delivery Value = %q, want %q", dst.Value(), "abc")
	}
}

func TestCommutativeRandomOrder(t *testing.T) {
	// Build a set of ops on a source, then apply them in many shuffled orders
	// to fresh replicas; all must converge to the same value.
	src := New(1)
	ops := typeString(src, 0, "collaborative")
	want := src.Value()

	rng := rand.New(rand.NewSource(42))
	for trial := 0; trial < 50; trial++ {
		shuffled := make([]Op, len(ops))
		copy(shuffled, ops)
		rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		d := New(uint64(100 + trial))
		applyAll(d, shuffled)
		if d.Value() != want {
			t.Fatalf("shuffle trial %d diverged: %q != %q", trial, d.Value(), want)
		}
	}
}

func TestConcurrentInterleaving(t *testing.T) {
	// Two sites edit independently, then merge both ways; must match.
	a := New(1)
	b := New(2)
	base := typeString(a, 0, "12345")
	applyAll(b, base)

	// a deletes '3' and appends 'A'; b inserts 'B' after '1'.
	da, _ := a.LocalDelete(2)   // "1245"
	ia := a.LocalInsert(4, 'A') // "1245A"
	ib := b.LocalInsert(1, 'B') // "1B2345"

	// merge
	applyAll(a, []Op{ib})
	applyAll(b, []Op{da, ia})

	if a.Value() != b.Value() {
		t.Fatalf("interleaving diverged: %q vs %q", a.Value(), b.Value())
	}
}

func TestDeleteThenRemoteInsertAtSamePlace(t *testing.T) {
	a := New(1)
	b := New(2)
	base := typeString(a, 0, "xy")
	applyAll(b, base)

	// a deletes 'x'; b concurrently inserts 'Z' after 'x'.
	da, _ := a.LocalDelete(0)
	ib := b.LocalInsert(1, 'Z') // b sees "xZy"

	b.Apply(da)
	a.Apply(ib)

	if a.Value() != b.Value() {
		t.Fatalf("diverged: %q vs %q", a.Value(), b.Value())
	}
	// 'x' is gone on both; 'Z' survives (tombstone keeps the tree position).
	if a.Value() != "Zy" {
		t.Errorf("Value = %q, want %q", a.Value(), "Zy")
	}
}
