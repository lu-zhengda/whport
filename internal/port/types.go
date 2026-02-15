package port

import "fmt"

// Protocol represents a network protocol.
type Protocol string

const (
	TCP Protocol = "TCP"
	UDP Protocol = "UDP"
)

// PortEntry represents a single port being used by a process.
type PortEntry struct {
	Port     int
	Protocol Protocol
	PID      int
	Process  string // short process name
	User     string // owner
	Command  string // full command path
	State    string // LISTEN, ESTABLISHED, etc.
	FD       string // file descriptor
}

// String returns a human-readable representation of the entry.
func (e PortEntry) String() string {
	return fmt.Sprintf("%d/%s (PID %d, %s)", e.Port, e.Protocol, e.PID, e.Process)
}
