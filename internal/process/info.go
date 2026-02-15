package process

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lu-zhengda/whport/internal/port"
)

// ProcessInfo holds detailed information about a running process.
type ProcessInfo struct {
	PID        int
	PPID       int
	Name       string
	Command    string // full command line
	User       string
	StartTime  time.Time
	CPUPercent float64
	MemRSS     int64 // in bytes
	State      string
	Children   []int // child PIDs
}

// InfoFetcher retrieves detailed process information.
type InfoFetcher struct {
	runner port.CmdRunner
}

// NewInfoFetcher creates a new InfoFetcher.
func NewInfoFetcher(runner port.CmdRunner) *InfoFetcher {
	return &InfoFetcher{runner: runner}
}

// GetInfo retrieves detailed information for a process.
func (f *InfoFetcher) GetInfo(ctx context.Context, pid int) (*ProcessInfo, error) {
	// Use ps to get process details.
	out, err := f.runner.Run(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "pid=,ppid=,user=,%cpu=,rss=,lstart=,command=")
	if err != nil {
		return nil, fmt.Errorf("failed to get process info for PID %d: %w", pid, err)
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return nil, fmt.Errorf("process %d not found", pid)
	}

	info, err := parsePsOutput(line)
	if err != nil {
		return nil, fmt.Errorf("failed to parse process info: %w", err)
	}

	// Get process name via shorter ps call.
	nameOut, err := f.runner.Run(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "comm=")
	if err == nil {
		name := strings.TrimSpace(string(nameOut))
		if name != "" {
			// Extract just the binary name from path.
			parts := strings.Split(name, "/")
			info.Name = parts[len(parts)-1]
		}
	}

	// Get children PIDs.
	childOut, err := f.runner.Run(ctx, "pgrep", "-P", strconv.Itoa(pid))
	if err == nil {
		info.Children = parseChildPIDs(string(childOut))
	}

	return info, nil
}

// parsePsOutput parses the output of ps -o pid=,ppid=,user=,%cpu=,rss=,lstart=,command=
func parsePsOutput(line string) (*ProcessInfo, error) {
	// Fields are space-separated, but lstart and command can contain spaces.
	// lstart format: "Day Mon DD HH:MM:SS YYYY" (5 tokens)
	// Layout: PID PPID USER %CPU RSS <lstart 5 tokens> COMMAND...
	fields := strings.Fields(line)
	if len(fields) < 11 {
		return nil, fmt.Errorf("unexpected ps output format: %q", line)
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse PID: %w", err)
	}

	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse PPID: %w", err)
	}

	user := fields[2]

	cpu, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		cpu = 0.0
	}

	rss, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		rss = 0
	}
	// RSS from ps is in kilobytes, convert to bytes.
	rssBytes := rss * 1024

	// Parse lstart: fields[5] through fields[9] e.g. "Thu Feb 13 10:30:00 2026"
	lstartStr := strings.Join(fields[5:10], " ")
	startTime, err := time.Parse("Mon Jan 2 15:04:05 2006", lstartStr)
	if err != nil {
		startTime = time.Time{}
	}

	// Everything from fields[10] onwards is the command.
	command := strings.Join(fields[10:], " ")

	return &ProcessInfo{
		PID:        pid,
		PPID:       ppid,
		User:       user,
		CPUPercent: cpu,
		MemRSS:     rssBytes,
		StartTime:  startTime,
		Command:    command,
	}, nil
}

// parseChildPIDs parses pgrep output into a slice of PIDs.
func parseChildPIDs(output string) []int {
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}
