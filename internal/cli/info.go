package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zhengda-lu/whport/internal/port"
	"github.com/zhengda-lu/whport/internal/process"
)

var infoCmd = &cobra.Command{
	Use:   "info <port>",
	Short: "Detailed info about a port and its process",
	Long:  "Display detailed information about the process listening on the specified port.",
	Args:  cobra.ExactArgs(1),
	RunE:  runInfo,
}

func runInfo(cmd *cobra.Command, args []string) error {
	portNum, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid port number: %w", err)
	}

	ctx := context.Background()
	runner := &port.RealCmdRunner{}
	scanner := port.NewLsofScanner(runner)
	manager := process.NewRealManager(runner)

	entries, err := scanner.FindByPort(ctx, portNum)
	if err != nil {
		return fmt.Errorf("failed to find processes on port %d: %w", portNum, err)
	}

	// Find the LISTEN entry.
	var target *port.PortEntry
	for _, e := range entries {
		if e.State == "LISTEN" && e.Port == portNum {
			e := e // capture
			target = &e
			break
		}
	}

	if target == nil {
		// Fall back to any entry on that port.
		for _, e := range entries {
			if e.Port == portNum {
				e := e
				target = &e
				break
			}
		}
	}

	if target == nil {
		return fmt.Errorf("no process found on port %d", portNum)
	}

	// Get detailed process info.
	info, err := manager.Info(ctx, target.PID)

	if jsonOutput {
		return printInfoJSON(target, info)
	}

	return printInfoHuman(target, info, err)
}

func printInfoHuman(entry *port.PortEntry, info *process.ProcessInfo, infoErr error) error {
	fmt.Printf("Port:        %d/%s\n", entry.Port, entry.Protocol)
	fmt.Printf("State:       %s\n", entry.State)
	fmt.Printf("Process:     %s (PID %d)\n", entry.Process, entry.PID)

	if info != nil {
		fmt.Printf("Command:     %s\n", info.Command)
		fmt.Printf("User:        %s\n", info.User)

		if !info.StartTime.IsZero() {
			ago := time.Since(info.StartTime).Truncate(time.Second)
			fmt.Printf("Started:     %s ago (%s)\n",
				formatDuration(ago),
				info.StartTime.Format("2006-01-02 15:04:05"))
		}

		fmt.Printf("CPU:         %.1f%%\n", info.CPUPercent)
		fmt.Printf("Memory:      %s (RSS)\n", formatBytes(info.MemRSS))

		if info.PPID > 0 {
			fmt.Printf("Parent PID:  %d\n", info.PPID)
		}

		if len(info.Children) > 0 {
			childStrs := make([]string, len(info.Children))
			for i, c := range info.Children {
				childStrs[i] = strconv.Itoa(c)
			}
			fmt.Printf("Children:    %s\n", strings.Join(childStrs, ", "))
		}
	} else {
		fmt.Printf("User:        %s\n", entry.User)
		if infoErr != nil {
			fmt.Printf("Details:     (unavailable: %v)\n", infoErr)
		}
	}

	return nil
}

func printInfoJSON(entry *port.PortEntry, info *process.ProcessInfo) error {
	type jsonInfo struct {
		Port       int      `json:"port"`
		Protocol   string   `json:"protocol"`
		State      string   `json:"state"`
		PID        int      `json:"pid"`
		Process    string   `json:"process"`
		Command    string   `json:"command,omitempty"`
		User       string   `json:"user"`
		StartTime  string   `json:"start_time,omitempty"`
		CPUPercent float64  `json:"cpu_percent,omitempty"`
		MemoryRSS  int64    `json:"memory_rss_bytes,omitempty"`
		PPID       int      `json:"ppid,omitempty"`
		Children   []int    `json:"children,omitempty"`
	}

	out := jsonInfo{
		Port:     entry.Port,
		Protocol: string(entry.Protocol),
		State:    entry.State,
		PID:      entry.PID,
		Process:  entry.Process,
		User:     entry.User,
	}

	if info != nil {
		out.Command = info.Command
		out.User = info.User
		out.CPUPercent = info.CPUPercent
		out.MemoryRSS = info.MemRSS
		out.PPID = info.PPID
		out.Children = info.Children
		if !info.StartTime.IsZero() {
			out.StartTime = info.StartTime.Format(time.RFC3339)
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%d hours", hours)
	}
	days := hours / 24
	return fmt.Sprintf("%d days %d hours", days, hours%24)
}

func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
