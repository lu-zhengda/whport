package process

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/zhengda-lu/whport/internal/port"
)

// protectedPIDs lists PIDs that should never be killed.
var protectedPIDs = map[int]bool{
	0: true,
	1: true,
}

// Manager provides process lifecycle management.
type Manager interface {
	Kill(pid int, signal syscall.Signal) error
	Info(ctx context.Context, pid int) (*ProcessInfo, error)
	IsRunning(pid int) bool
}

// RealManager implements Manager using real system calls.
type RealManager struct {
	runner  port.CmdRunner
	fetcher *InfoFetcher
}

// NewRealManager creates a new process manager.
func NewRealManager(runner port.CmdRunner) *RealManager {
	return &RealManager{
		runner:  runner,
		fetcher: NewInfoFetcher(runner),
	}
}

// Kill sends a signal to a process. It refuses to kill protected PIDs.
func (m *RealManager) Kill(pid int, signal syscall.Signal) error {
	if protectedPIDs[pid] {
		return fmt.Errorf("refusing to kill protected PID %d", pid)
	}

	if !m.IsRunning(pid) {
		return fmt.Errorf("process %d is not running", pid)
	}

	if err := syscall.Kill(pid, signal); err != nil {
		return fmt.Errorf("failed to send signal %d to PID %d: %w", signal, pid, err)
	}

	return nil
}

// GracefulKill sends SIGTERM, waits up to 3 seconds, then returns whether
// the process exited. The caller can then decide to SIGKILL.
func (m *RealManager) GracefulKill(pid int) (exited bool, err error) {
	if err := m.Kill(pid, syscall.SIGTERM); err != nil {
		return false, err
	}

	// Poll for up to 3 seconds.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !m.IsRunning(pid) {
			return true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return !m.IsRunning(pid), nil
}

// ForceKill sends SIGKILL to a process.
func (m *RealManager) ForceKill(pid int) error {
	return m.Kill(pid, syscall.SIGKILL)
}

// Info retrieves detailed process information.
func (m *RealManager) Info(ctx context.Context, pid int) (*ProcessInfo, error) {
	return m.fetcher.GetInfo(ctx, pid)
}

// IsRunning checks if a process with the given PID exists.
func (m *RealManager) IsRunning(pid int) bool {
	// On Unix, sending signal 0 checks if the process exists.
	return syscall.Kill(pid, 0) == nil
}

// VerifyProcess checks if a PID still corresponds to the expected process
// by comparing the command name.
func (m *RealManager) VerifyProcess(ctx context.Context, pid int, expectedName string) bool {
	out, err := m.runner.Run(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	if err != nil {
		return false
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return false
	}
	// Extract just the binary name from path.
	parts := strings.Split(name, "/")
	actual := parts[len(parts)-1]
	return strings.EqualFold(actual, expectedName)
}
