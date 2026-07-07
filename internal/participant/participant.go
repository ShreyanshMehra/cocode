// Package participant provides a concurrency-safe registry for collaborator presence.
package participant

import (
	"sort"
	"sync"
)

// Participant describes a collaborator's assigned identity and cursor position.
type Participant struct {
	ID     string `json:"id"`     // caller-provided stable id (e.g. connection id)
	Name   string `json:"name"`   // assigned display name, e.g. "Blue Fox"
	Color  string `json:"color"`  // assigned hex color, e.g. "#4f8cff"
	Cursor int    `json:"cursor"` // caret index in the document
}

type paletteEntry struct {
	name  string
	color string
}

var palette = []paletteEntry{
	{name: "Blue Fox", color: "#4f8cff"},
	{name: "Green Owl", color: "#34c759"},
	{name: "Purple Cat", color: "#af52de"},
	{name: "Orange Bear", color: "#ff9500"},
	{name: "Pink Dolphin", color: "#ff2d55"},
	{name: "Teal Turtle", color: "#30b0c7"},
	{name: "Red Panda", color: "#ff3b30"},
	{name: "Yellow Lion", color: "#ffcc00"},
}

// Registry tracks active participants and their cursor positions.
type Registry struct {
	mu           sync.RWMutex
	participants map[string]*Participant
	counter      int
}

// NewRegistry creates an empty participant registry.
func NewRegistry() *Registry {
	return &Registry{
		participants: make(map[string]*Participant),
	}
}

// Join registers a new participant by id and returns the assigned record.
// Re-joining with the same id returns the existing record.
func (r *Registry) Join(id string) Participant {
	r.mu.Lock()
	defer r.mu.Unlock()

	if participant, ok := r.participants[id]; ok {
		return *participant
	}

	entry := palette[r.counter%len(palette)]
	r.counter++
	participant := &Participant{
		ID:    id,
		Name:  entry.name,
		Color: entry.color,
	}
	r.participants[id] = participant

	return *participant
}

// Leave removes a participant. It is safe to call for an unknown id.
func (r *Registry) Leave(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.participants, id)
}

// SetCursor updates a participant's cursor index. It is a no-op for an unknown id.
func (r *Registry) SetCursor(id string, pos int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if participant, ok := r.participants[id]; ok {
		participant.Cursor = pos
	}
}

// List returns a snapshot of all participants sorted by Name for deterministic output.
func (r *Registry) List() []Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()

	participants := make([]Participant, 0, len(r.participants))
	for _, participant := range r.participants {
		participants = append(participants, *participant)
	}

	sort.Slice(participants, func(i, j int) bool {
		if participants[i].Name == participants[j].Name {
			return participants[i].ID < participants[j].ID
		}
		return participants[i].Name < participants[j].Name
	})

	return participants
}

// Count returns the number of active participants.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.participants)
}
