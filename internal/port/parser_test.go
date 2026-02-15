package port

import (
	"testing"
)

func TestParseLsofOutput(t *testing.T) {
	input := `COMMAND     PID      USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
nginx      1234      root    6u  IPv4 0x1234567890      0t0  TCP *:80 (LISTEN)
nginx      1234      root    7u  IPv4 0x1234567891      0t0  TCP *:443 (LISTEN)
node       5678   zhengda    8u  IPv6 0x1234567892      0t0  TCP *:3000 (LISTEN)
postgres   9012 _postgres    9u  IPv4 0x1234567893      0t0  TCP 127.0.0.1:5432 (LISTEN)
java       3456   zhengda   10u  IPv4 0x1234567894      0t0  TCP *:8080 (LISTEN)
`

	entries := ParseLsofOutput(input)

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	tests := []struct {
		idx     int
		process string
		pid     int
		user    string
		port    int
		proto   Protocol
		state   string
	}{
		{0, "nginx", 1234, "root", 80, TCP, "LISTEN"},
		{1, "nginx", 1234, "root", 443, TCP, "LISTEN"},
		{2, "node", 5678, "zhengda", 3000, TCP, "LISTEN"},
		{3, "postgres", 9012, "_postgres", 5432, TCP, "LISTEN"},
		{4, "java", 3456, "zhengda", 8080, TCP, "LISTEN"},
	}

	for _, tt := range tests {
		e := entries[tt.idx]
		if e.Process != tt.process {
			t.Errorf("[%d] process: got %q, want %q", tt.idx, e.Process, tt.process)
		}
		if e.PID != tt.pid {
			t.Errorf("[%d] pid: got %d, want %d", tt.idx, e.PID, tt.pid)
		}
		if e.User != tt.user {
			t.Errorf("[%d] user: got %q, want %q", tt.idx, e.User, tt.user)
		}
		if e.Port != tt.port {
			t.Errorf("[%d] port: got %d, want %d", tt.idx, e.Port, tt.port)
		}
		if e.Protocol != tt.proto {
			t.Errorf("[%d] protocol: got %q, want %q", tt.idx, e.Protocol, tt.proto)
		}
		if e.State != tt.state {
			t.Errorf("[%d] state: got %q, want %q", tt.idx, e.State, tt.state)
		}
	}
}

func TestParseLsofOutput_Established(t *testing.T) {
	input := `COMMAND     PID      USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
chrome     1111   zhengda   20u  IPv4 0x1234567890      0t0  TCP 192.168.1.10:54321->93.184.216.34:443 (ESTABLISHED)
`

	entries := ParseLsofOutput(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Port != 54321 {
		t.Errorf("port: got %d, want 54321", e.Port)
	}
	if e.State != "ESTABLISHED" {
		t.Errorf("state: got %q, want ESTABLISHED", e.State)
	}
	if e.Process != "chrome" {
		t.Errorf("process: got %q, want chrome", e.Process)
	}
}

func TestParseLsofOutput_UDP(t *testing.T) {
	input := `COMMAND     PID      USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
mDNSRespo   100      root    5u  IPv4 0x1234567890      0t0  UDP *:5353
`

	entries := ParseLsofOutput(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Protocol != UDP {
		t.Errorf("protocol: got %q, want UDP", e.Protocol)
	}
	if e.Port != 5353 {
		t.Errorf("port: got %d, want 5353", e.Port)
	}
}

func TestParseLsofOutput_EmptyInput(t *testing.T) {
	entries := ParseLsofOutput("")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseLsofOutput_HeaderOnly(t *testing.T) {
	input := `COMMAND     PID      USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
`
	entries := ParseLsofOutput(input)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseNameField(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		proto    Protocol
		wantPort int
		wantSt   string
	}{
		{"listen wildcard", "*:8080", TCP, 8080, "LISTEN"},
		{"listen localhost", "127.0.0.1:3000", TCP, 3000, "LISTEN"},
		{"listen with state", "*:443 (LISTEN)", TCP, 443, "LISTEN"},
		{"established", "192.168.1.10:54321->93.184.216.34:443", TCP, 54321, "ESTABLISHED"},
		{"wildcard star", "*:*", TCP, -1, ""},
		{"udp port", "*:5353", UDP, 5353, "LISTEN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, state := parseNameField(tt.input, tt.proto)
			if port != tt.wantPort {
				t.Errorf("port: got %d, want %d", port, tt.wantPort)
			}
			if state != tt.wantSt {
				t.Errorf("state: got %q, want %q", state, tt.wantSt)
			}
		})
	}
}
