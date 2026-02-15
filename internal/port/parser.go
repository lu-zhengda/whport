package port

import (
	"strconv"
	"strings"
)

// ParseLsofOutput parses the columnar output from lsof -iTCP -iUDP -P -n.
// Each line after the header has fields: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME
func ParseLsofOutput(output string) []PortEntry {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return nil
	}

	var entries []PortEntry
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		entry, ok := parseLsofLine(line)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// parseLsofLine parses a single lsof output line into a PortEntry.
// Format: COMMAND  PID  USER  FD  TYPE  DEVICE  SIZE/OFF  NODE  NAME
func parseLsofLine(line string) (PortEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return PortEntry{}, false
	}

	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return PortEntry{}, false
	}

	proto := parseProtocol(fields[7])
	port, state := parseNameField(fields[8], proto)
	if port < 0 {
		return PortEntry{}, false
	}

	return PortEntry{
		Process:  fields[0],
		PID:      pid,
		User:     fields[2],
		FD:       fields[3],
		Protocol: proto,
		Port:     port,
		State:    state,
		Command:  fields[0], // will be enriched later via ps
	}, true
}

// parseProtocol converts the NODE field to a Protocol.
func parseProtocol(node string) Protocol {
	upper := strings.ToUpper(node)
	if strings.Contains(upper, "UDP") {
		return UDP
	}
	return TCP
}

// parseNameField extracts the port number and connection state from the NAME field.
// NAME formats:
//   - "*:8080" or "127.0.0.1:8080" (LISTEN implied)
//   - "127.0.0.1:8080->127.0.0.1:54321" (ESTABLISHED)
//   - "*:8080 (LISTEN)" or similar with state in parentheses
//
// For connections with "->", we extract the local port (left side).
func parseNameField(name string, proto Protocol) (int, string) {
	state := ""

	// Check for state in parentheses at the end.
	if idx := strings.LastIndex(name, "("); idx != -1 {
		closeParen := strings.LastIndex(name, ")")
		if closeParen > idx {
			state = name[idx+1 : closeParen]
			name = strings.TrimSpace(name[:idx])
		}
	}

	// Split on "->" for established connections.
	local := name
	if idx := strings.Index(name, "->"); idx != -1 {
		local = name[:idx]
		if state == "" {
			state = "ESTABLISHED"
		}
	}

	// Extract port from local address.
	portStr := local
	if idx := strings.LastIndex(local, ":"); idx != -1 {
		portStr = local[idx+1:]
	}

	// Handle wildcard or port-only entries.
	if portStr == "*" {
		return -1, ""
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return -1, ""
	}

	if state == "" {
		state = "LISTEN"
	}

	return port, state
}
