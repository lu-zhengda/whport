package port

import (
	"context"
	"fmt"
	"strings"
)

// Scanner defines the interface for discovering ports and their processes.
type Scanner interface {
	ListPorts(ctx context.Context) ([]PortEntry, error)
	FindByPort(ctx context.Context, port int) ([]PortEntry, error)
	FindByProcess(ctx context.Context, name string) ([]PortEntry, error)
}

// CmdRunner abstracts shell command execution for testability.
type CmdRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// LsofScanner implements Scanner using macOS lsof.
type LsofScanner struct {
	runner CmdRunner
}

// NewLsofScanner creates a new scanner backed by lsof.
func NewLsofScanner(runner CmdRunner) *LsofScanner {
	return &LsofScanner{runner: runner}
}

// ListPorts returns all listening ports.
func (s *LsofScanner) ListPorts(ctx context.Context) ([]PortEntry, error) {
	out, err := s.runner.Run(ctx, "lsof", "-iTCP", "-iUDP", "-sTCP:LISTEN", "-P", "-n")
	if err != nil {
		return nil, fmt.Errorf("failed to run lsof: %w", err)
	}
	return ParseLsofOutput(string(out)), nil
}

// ListAllPorts returns all connections including ESTABLISHED.
func (s *LsofScanner) ListAllPorts(ctx context.Context) ([]PortEntry, error) {
	out, err := s.runner.Run(ctx, "lsof", "-iTCP", "-iUDP", "-P", "-n")
	if err != nil {
		return nil, fmt.Errorf("failed to run lsof: %w", err)
	}
	return ParseLsofOutput(string(out)), nil
}

// FindByPort returns all entries matching the given port number.
func (s *LsofScanner) FindByPort(ctx context.Context, port int) ([]PortEntry, error) {
	out, err := s.runner.Run(ctx, "lsof", fmt.Sprintf("-i:%d", port), "-P", "-n")
	if err != nil {
		return nil, fmt.Errorf("failed to run lsof for port %d: %w", port, err)
	}
	return ParseLsofOutput(string(out)), nil
}

// FindByProcess returns all entries matching the given process name.
func (s *LsofScanner) FindByProcess(ctx context.Context, name string) ([]PortEntry, error) {
	entries, err := s.ListPorts(ctx)
	if err != nil {
		return nil, err
	}

	var matched []PortEntry
	lower := strings.ToLower(name)
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Process), lower) ||
			strings.Contains(strings.ToLower(e.Command), lower) {
			matched = append(matched, e)
		}
	}
	return matched, nil
}
