package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/lu-zhengda/whport/internal/port"
)

// EventType describes what happened to a port.
type EventType string

const (
	EventOpen  EventType = "open"
	EventClose EventType = "close"
)

// Event represents a single port open or close event.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Type      EventType `json:"type"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"`
	PID       int       `json:"pid"`
	Process   string    `json:"process"`
	User      string    `json:"user"`
}

// Snapshot represents the ports that were open at a given point in time.
type Snapshot struct {
	Timestamp time.Time       `json:"timestamp"`
	Entries   []SnapshotEntry `json:"entries"`
}

// SnapshotEntry is a simplified port entry for snapshot storage.
type SnapshotEntry struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	PID      int    `json:"pid"`
	Process  string `json:"process"`
	User     string `json:"user"`
}

// Store manages history persistence at ~/.config/whport/history.json.
type Store struct {
	path string
}

// Data is the on-disk format for the history file.
type Data struct {
	LastSnapshot *Snapshot `json:"last_snapshot,omitempty"`
	Events       []Event   `json:"events"`
}

// NewStore creates a Store with the default path.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	return &Store{
		path: filepath.Join(home, ".config", "whport", "history.json"),
	}, nil
}

// NewStoreWithPath creates a Store at the given path (useful for testing).
func NewStoreWithPath(path string) *Store {
	return &Store{path: path}
}

// Load reads the history data from disk. Returns empty data if file doesn't exist.
func (s *Store) Load() (*Data, error) {
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return &Data{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	var data Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to parse history file: %w", err)
	}
	return &data, nil
}

// Save writes the history data to disk, creating parent directories as needed.
func (s *Store) Save(data *Data) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history data: %w", err)
	}

	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}
	return nil
}

// SnapshotFromEntries converts port.PortEntry slice to a Snapshot.
func SnapshotFromEntries(entries []port.PortEntry, ts time.Time) *Snapshot {
	snap := &Snapshot{
		Timestamp: ts,
		Entries:   make([]SnapshotEntry, len(entries)),
	}
	for i, e := range entries {
		snap.Entries[i] = SnapshotEntry{
			Port:     e.Port,
			Protocol: string(e.Protocol),
			PID:      e.PID,
			Process:  e.Process,
			User:     e.User,
		}
	}
	return snap
}

// portKey creates a unique key for a port entry to detect changes.
func portKey(port int, protocol string) string {
	return fmt.Sprintf("%d/%s", port, protocol)
}

// snapshotEntryKey returns the key for a SnapshotEntry.
func snapshotEntryKey(e SnapshotEntry) string {
	return portKey(e.Port, e.Protocol)
}

// Diff compares a previous snapshot to the current entries and returns events
// for ports that opened or closed.
func Diff(prev *Snapshot, current []port.PortEntry, ts time.Time) []Event {
	// Build maps of previous and current states.
	prevMap := make(map[string]SnapshotEntry)
	if prev != nil {
		for _, e := range prev.Entries {
			prevMap[snapshotEntryKey(e)] = e
		}
	}

	currMap := make(map[string]port.PortEntry)
	for _, e := range current {
		key := portKey(e.Port, string(e.Protocol))
		currMap[key] = e
	}

	var events []Event

	// New ports (in current but not in previous) = open events.
	for key, e := range currMap {
		if _, existed := prevMap[key]; !existed {
			events = append(events, Event{
				Timestamp: ts,
				Type:      EventOpen,
				Port:      e.Port,
				Protocol:  string(e.Protocol),
				PID:       e.PID,
				Process:   e.Process,
				User:      e.User,
			})
		}
	}

	// Gone ports (in previous but not in current) = close events.
	for key, e := range prevMap {
		if _, exists := currMap[key]; !exists {
			events = append(events, Event{
				Timestamp: ts,
				Type:      EventClose,
				Port:      e.Port,
				Protocol:  e.Protocol,
				PID:       e.PID,
				Process:   e.Process,
				User:      e.User,
			})
		}
	}

	// Sort events by port for deterministic output.
	sort.Slice(events, func(i, j int) bool {
		if events[i].Port != events[j].Port {
			return events[i].Port < events[j].Port
		}
		return events[i].Type < events[j].Type
	})

	return events
}

// Record takes the current port entries, diffs against the stored snapshot,
// appends new events, updates the snapshot, and saves.
func (s *Store) Record(entries []port.PortEntry, ts time.Time) ([]Event, error) {
	data, err := s.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load history: %w", err)
	}

	events := Diff(data.LastSnapshot, entries, ts)
	data.Events = append(data.Events, events...)
	data.LastSnapshot = SnapshotFromEntries(entries, ts)

	if err := s.Save(data); err != nil {
		return nil, fmt.Errorf("failed to save history: %w", err)
	}

	return events, nil
}
