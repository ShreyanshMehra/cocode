package participant

import (
	"fmt"
	"sort"
	"sync"
	"testing"
)

func TestJoinAssignsDeterministicNameAndColor(t *testing.T) {
	registry := NewRegistry()

	participants := []Participant{
		registry.Join("first"),
		registry.Join("second"),
		registry.Join("third"),
	}

	for i, participant := range participants {
		if participant.Name != palette[i].name {
			t.Fatalf("participant %d name = %q, want %q", i, participant.Name, palette[i].name)
		}
		if participant.Color != palette[i].color {
			t.Fatalf("participant %d color = %q, want %q", i, participant.Color, palette[i].color)
		}
	}
}

func TestJoinIsIdempotent(t *testing.T) {
	registry := NewRegistry()

	first := registry.Join("same")
	second := registry.Join("same")

	if first != second {
		t.Fatalf("second Join returned %+v, want %+v", second, first)
	}
	if count := registry.Count(); count != 1 {
		t.Fatalf("Count() = %d, want 1", count)
	}
}

func TestSetCursorUpdatesAndUnknownIsNoOp(t *testing.T) {
	registry := NewRegistry()
	registry.Join("known")

	registry.SetCursor("known", 42)
	registry.SetCursor("unknown", 99)

	participants := registry.List()
	if len(participants) != 1 {
		t.Fatalf("List() length = %d, want 1", len(participants))
	}
	if participants[0].Cursor != 42 {
		t.Fatalf("Cursor = %d, want 42", participants[0].Cursor)
	}
	if count := registry.Count(); count != 1 {
		t.Fatalf("Count() = %d, want 1", count)
	}
}

func TestLeaveRemovesParticipantAndUnknownIsSafe(t *testing.T) {
	registry := NewRegistry()
	registry.Join("one")
	registry.Join("two")

	registry.Leave("one")
	if count := registry.Count(); count != 1 {
		t.Fatalf("Count() after Leave = %d, want 1", count)
	}

	registry.Leave("unknown")
	if count := registry.Count(); count != 1 {
		t.Fatalf("Count() after unknown Leave = %d, want 1", count)
	}

	participants := registry.List()
	if len(participants) != 1 || participants[0].ID != "two" {
		t.Fatalf("List() = %+v, want only id two", participants)
	}
}

func TestListSortedByNameAndSnapshot(t *testing.T) {
	registry := NewRegistry()
	for _, id := range []string{"one", "two", "three", "four"} {
		registry.Join(id)
	}

	participants := registry.List()
	if !sort.SliceIsSorted(participants, func(i, j int) bool {
		if participants[i].Name == participants[j].Name {
			return participants[i].ID < participants[j].ID
		}
		return participants[i].Name < participants[j].Name
	}) {
		t.Fatalf("List() is not sorted by Name: %+v", participants)
	}

	participants[0].Name = "Mutated Name"
	participants[0].Cursor = 999

	fresh := registry.List()
	for _, participant := range fresh {
		if participant.Name == "Mutated Name" || participant.Cursor == 999 {
			t.Fatalf("List() did not return a snapshot: %+v", fresh)
		}
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	registry := NewRegistry()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			id := fmt.Sprintf("participant-%02d", i)
			registry.Join(id)
			registry.SetCursor(id, i)
			_ = registry.List()
			_ = registry.Count()
		}()
	}
	wg.Wait()

	if count := registry.Count(); count != goroutines {
		t.Fatalf("Count() = %d, want %d", count, goroutines)
	}

	participants := registry.List()
	if len(participants) != goroutines {
		t.Fatalf("List() length = %d, want %d", len(participants), goroutines)
	}
}
