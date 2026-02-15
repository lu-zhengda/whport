package port

import (
	"context"
	"io"
	"os/exec"
	"strings"
)

// RealCmdRunner executes real shell commands.
type RealCmdRunner struct{}

// Run executes a command and returns its stdout. Stderr is suppressed
// to prevent it from leaking into TUI output.
func (r *RealCmdRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = io.Discard
	return cmd.Output()
}

// MockCmdRunner returns canned responses for testing.
type MockCmdRunner struct {
	Output []byte
	Err    error
}

// Run returns the pre-configured output and error.
func (m *MockCmdRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return m.Output, m.Err
}

// MultiMockCmdRunner returns different responses based on the command.
// Keys are "name arg1 arg2 ..." strings.
type MultiMockCmdRunner struct {
	Responses map[string]MockResponse
}

// MockResponse holds a single command's output and error.
type MockResponse struct {
	Output []byte
	Err    error
}

// Run looks up the command key and returns its pre-configured response.
// Falls back to empty output and nil error if no match is found.
func (m *MultiMockCmdRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name
	if len(args) > 0 {
		key = name + " " + strings.Join(args, " ")
	}
	if resp, ok := m.Responses[key]; ok {
		return resp.Output, resp.Err
	}
	return nil, nil
}
