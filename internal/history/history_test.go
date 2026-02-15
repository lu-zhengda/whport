package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lu-zhengda/whport/internal/port"
)

func TestDiff_NoPrevious(t *testing.T) {
	ts := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	current := []port.PortEntry{
		{Port: 80, Protocol: port.TCP, PID: 100, Process: "nginx", User: "root"},
		{Port: 3000, Protocol: port.TCP, PID: 200, Process: "node", User: "zhengda"},
	}

	events := Diff(nil, current, ts)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Events should be sorted by port.
	if events[0].Port != 80 || events[0].Type != EventOpen {
		t.Errorf("event[0]: got port=%d type=%s, want port=80 type=open", events[0].Port, events[0].Type)
	}
	if events[1].Port != 3000 || events[1].Type != EventOpen {
		t.Errorf("event[1]: got port=%d type=%s, want port=3000 type=open", events[1].Port, events[1].Type)
	}
}

func TestDiff_NoChanges(t *testing.T) {
	ts := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	prev := &Snapshot{
		Timestamp: ts.Add(-time.Minute),
		Entries: []SnapshotEntry{
			{Port: 80, Protocol: "TCP", PID: 100, Process: "nginx", User: "root"},
		},
	}
	current := []port.PortEntry{
		{Port: 80, Protocol: port.TCP, PID: 100, Process: "nginx", User: "root"},
	}

	events := Diff(prev, current, ts)

	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestDiff_PortOpened(t *testing.T) {
	ts := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	prev := &Snapshot{
		Timestamp: ts.Add(-time.Minute),
		Entries: []SnapshotEntry{
			{Port: 80, Protocol: "TCP", PID: 100, Process: "nginx", User: "root"},
		},
	}
	current := []port.PortEntry{
		{Port: 80, Protocol: port.TCP, PID: 100, Process: "nginx", User: "root"},
		{Port: 3000, Protocol: port.TCP, PID: 200, Process: "node", User: "zhengda"},
	}

	events := Diff(prev, current, ts)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventOpen || events[0].Port != 3000 {
		t.Errorf("expected open event on port 3000, got type=%s port=%d", events[0].Type, events[0].Port)
	}
	if events[0].Process != "node" {
		t.Errorf("process: got %q, want %q", events[0].Process, "node")
	}
}

func TestDiff_PortClosed(t *testing.T) {
	ts := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	prev := &Snapshot{
		Timestamp: ts.Add(-time.Minute),
		Entries: []SnapshotEntry{
			{Port: 80, Protocol: "TCP", PID: 100, Process: "nginx", User: "root"},
			{Port: 3000, Protocol: "TCP", PID: 200, Process: "node", User: "zhengda"},
		},
	}
	current := []port.PortEntry{
		{Port: 80, Protocol: port.TCP, PID: 100, Process: "nginx", User: "root"},
	}

	events := Diff(prev, current, ts)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventClose || events[0].Port != 3000 {
		t.Errorf("expected close event on port 3000, got type=%s port=%d", events[0].Type, events[0].Port)
	}
}

func TestDiff_MixedChanges(t *testing.T) {
	ts := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	prev := &Snapshot{
		Timestamp: ts.Add(-time.Minute),
		Entries: []SnapshotEntry{
			{Port: 80, Protocol: "TCP", PID: 100, Process: "nginx", User: "root"},
			{Port: 5432, Protocol: "TCP", PID: 300, Process: "postgres", User: "_postgres"},
		},
	}
	current := []port.PortEntry{
		{Port: 80, Protocol: port.TCP, PID: 100, Process: "nginx", User: "root"},
		{Port: 3000, Protocol: port.TCP, PID: 200, Process: "node", User: "zhengda"},
	}

	events := Diff(prev, current, ts)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Sorted by port: 3000 (open), 5432 (close).
	if events[0].Type != EventOpen || events[0].Port != 3000 {
		t.Errorf("event[0]: got type=%s port=%d, want open 3000", events[0].Type, events[0].Port)
	}
	if events[1].Type != EventClose || events[1].Port != 5432 {
		t.Errorf("event[1]: got type=%s port=%d, want close 5432", events[1].Type, events[1].Port)
	}
}

func TestDiff_DifferentProtocols(t *testing.T) {
	ts := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	prev := &Snapshot{
		Timestamp: ts.Add(-time.Minute),
		Entries: []SnapshotEntry{
			{Port: 53, Protocol: "TCP", PID: 100, Process: "dnsmasq", User: "root"},
		},
	}
	current := []port.PortEntry{
		{Port: 53, Protocol: port.TCP, PID: 100, Process: "dnsmasq", User: "root"},
		{Port: 53, Protocol: port.UDP, PID: 100, Process: "dnsmasq", User: "root"},
	}

	events := Diff(prev, current, ts)

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Protocol != "UDP" {
		t.Errorf("expected UDP protocol, got %s", events[0].Protocol)
	}
}

func TestSnapshotFromEntries(t *testing.T) {
	ts := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	entries := []port.PortEntry{
		{Port: 80, Protocol: port.TCP, PID: 100, Process: "nginx", User: "root", Command: "/usr/sbin/nginx", State: "LISTEN"},
	}

	snap := SnapshotFromEntries(entries, ts)

	if snap.Timestamp != ts {
		t.Errorf("timestamp: got %v, want %v", snap.Timestamp, ts)
	}
	if len(snap.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(snap.Entries))
	}
	if snap.Entries[0].Port != 80 {
		t.Errorf("port: got %d, want 80", snap.Entries[0].Port)
	}
	if snap.Entries[0].Protocol != "TCP" {
		t.Errorf("protocol: got %s, want TCP", snap.Entries[0].Protocol)
	}
}

func TestStore_LoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	store := NewStoreWithPath(path)

	// Load from non-existent file returns empty data.
	data, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected error loading empty store: %v", err)
	}
	if len(data.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(data.Events))
	}
	if data.LastSnapshot != nil {
		t.Errorf("expected nil snapshot, got %v", data.LastSnapshot)
	}

	// Save some data.
	ts := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	data.Events = []Event{
		{Timestamp: ts, Type: EventOpen, Port: 80, Protocol: "TCP", PID: 100, Process: "nginx", User: "root"},
	}
	data.LastSnapshot = &Snapshot{
		Timestamp: ts,
		Entries: []SnapshotEntry{
			{Port: 80, Protocol: "TCP", PID: 100, Process: "nginx", User: "root"},
		},
	}

	if err := store.Save(data); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	// Reload and verify.
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded.Events))
	}
	if loaded.Events[0].Port != 80 {
		t.Errorf("event port: got %d, want 80", loaded.Events[0].Port)
	}
	if loaded.LastSnapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(loaded.LastSnapshot.Entries) != 1 {
		t.Fatalf("expected 1 snapshot entry, got %d", len(loaded.LastSnapshot.Entries))
	}
}

func TestStore_Record(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	store := NewStoreWithPath(path)

	ts1 := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	entries1 := []port.PortEntry{
		{Port: 80, Protocol: port.TCP, PID: 100, Process: "nginx", User: "root"},
	}

	events1, err := store.Record(entries1, ts1)
	if err != nil {
		t.Fatalf("unexpected error on first record: %v", err)
	}
	if len(events1) != 1 {
		t.Fatalf("expected 1 event on first record, got %d", len(events1))
	}
	if events1[0].Type != EventOpen {
		t.Errorf("expected open event, got %s", events1[0].Type)
	}

	// Second record: port 80 still open, port 3000 new.
	ts2 := time.Date(2026, 2, 15, 10, 1, 0, 0, time.UTC)
	entries2 := []port.PortEntry{
		{Port: 80, Protocol: port.TCP, PID: 100, Process: "nginx", User: "root"},
		{Port: 3000, Protocol: port.TCP, PID: 200, Process: "node", User: "zhengda"},
	}

	events2, err := store.Record(entries2, ts2)
	if err != nil {
		t.Fatalf("unexpected error on second record: %v", err)
	}
	if len(events2) != 1 {
		t.Fatalf("expected 1 event on second record, got %d", len(events2))
	}
	if events2[0].Type != EventOpen || events2[0].Port != 3000 {
		t.Errorf("expected open event on port 3000, got type=%s port=%d", events2[0].Type, events2[0].Port)
	}

	// Verify cumulative events in store.
	data, err := store.Load()
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if len(data.Events) != 2 {
		t.Errorf("expected 2 cumulative events, got %d", len(data.Events))
	}
}

func TestStore_SaveCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "history.json")
	store := NewStoreWithPath(path)

	data := &Data{Events: []Event{}}
	if err := store.Save(data); err != nil {
		t.Fatalf("expected save to create directories, got error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected history file to exist after save")
	}
}

func TestStore_LoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStoreWithPath(path)
	_, err := store.Load()
	if err == nil {
		t.Fatal("expected error loading corrupt file")
	}
}
