package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/lu-zhengda/whport/internal/port"
	"github.com/spf13/cobra"
)

var (
	watchInterval int
	watchAlert    bool
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Auto-refresh port table in terminal",
	Long: `Continuously display listening ports with periodic refresh.

With --alert, monitors for new port listeners that appear after the initial
scan. When a new listener is detected, prints an alert and exits with code 1.
Useful for security monitoring.`,
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().IntVar(&watchInterval, "interval", 2, "Refresh interval in seconds")
	watchCmd.Flags().IntVar(&filterPort, "port", 0, "Filter by port number")
	watchCmd.Flags().StringVar(&filterProc, "process", "", "Filter by process name")
	watchCmd.Flags().StringVar(&filterProto, "protocol", "", "Filter by protocol (tcp/udp)")
	watchCmd.Flags().BoolVar(&watchAlert, "alert", false, "Alert and exit on new port listeners")
}

func runWatch(cmd *cobra.Command, args []string) error {
	if watchAlert {
		return runWatchAlert(cmd, args)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	runner := &port.RealCmdRunner{}
	scanner := port.NewLsofScanner(runner)
	interval := time.Duration(watchInterval) * time.Second

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial scan.
	if err := watchOnce(ctx, scanner); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nStopped watching.")
			return nil
		case <-ticker.C:
			if err := watchOnce(ctx, scanner); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		}
	}
}

func runWatchAlert(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	runner := &port.RealCmdRunner{}
	scanner := port.NewLsofScanner(runner)
	interval := time.Duration(watchInterval) * time.Second

	// Baseline scan.
	baseline, err := scanFiltered(ctx, scanner)
	if err != nil {
		return err
	}

	baselineKeys := makePortKeySet(baseline)

	if !jsonOutput {
		fmt.Printf("Monitoring %d port(s) for new listeners... (interval: %ds)\n",
			len(baseline), watchInterval)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if !jsonOutput {
				fmt.Println("\nStopped watching.")
			}
			return nil
		case <-ticker.C:
			current, err := scanFiltered(ctx, scanner)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				continue
			}

			newEntries := findNewEntries(current, baselineKeys)
			if len(newEntries) > 0 {
				if jsonOutput {
					return printAlertJSON(newEntries)
				}
				return printAlertHuman(newEntries)
			}
		}
	}
}

// scanFiltered performs a port scan and applies the current filters.
func scanFiltered(ctx context.Context, scanner *port.LsofScanner) ([]port.PortEntry, error) {
	var entries []port.PortEntry
	var err error

	if listAll {
		entries, err = scanner.ListAllPorts(ctx)
	} else {
		entries, err = scanner.ListPorts(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan ports: %w", err)
	}

	return filterEntries(entries), nil
}

// portKeyStr creates a unique key for identifying a port listener.
func portKeyStr(e port.PortEntry) string {
	return fmt.Sprintf("%d/%s", e.Port, e.Protocol)
}

// makePortKeySet builds a set of port keys from entries.
func makePortKeySet(entries []port.PortEntry) map[string]struct{} {
	keys := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		keys[portKeyStr(e)] = struct{}{}
	}
	return keys
}

// findNewEntries returns entries not present in the baseline set.
func findNewEntries(current []port.PortEntry, baseline map[string]struct{}) []port.PortEntry {
	var newEntries []port.PortEntry
	for _, e := range current {
		if _, exists := baseline[portKeyStr(e)]; !exists {
			newEntries = append(newEntries, e)
		}
	}
	return newEntries
}

// alertExitError is returned when --alert detects new ports.
// The CLI should exit with code 1.
type alertExitError struct {
	count int
}

func (e *alertExitError) Error() string {
	return fmt.Sprintf("alert: %d new port listener(s) detected", e.count)
}

func printAlertJSON(entries []port.PortEntry) error {
	type alertEntry struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		PID      int    `json:"pid"`
		Process  string `json:"process"`
		User     string `json:"user"`
		State    string `json:"state"`
		Command  string `json:"command"`
	}

	type alertOutput struct {
		Alert   string       `json:"alert"`
		Count   int          `json:"count"`
		Entries []alertEntry `json:"entries"`
	}

	out := alertOutput{
		Alert:   "new_port_listeners",
		Count:   len(entries),
		Entries: make([]alertEntry, len(entries)),
	}
	for i, e := range entries {
		out.Entries[i] = alertEntry{
			Port:     e.Port,
			Protocol: string(e.Protocol),
			PID:      e.PID,
			Process:  e.Process,
			User:     e.User,
			State:    e.State,
			Command:  e.Command,
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("failed to encode alert JSON: %w", err)
	}

	return &alertExitError{count: len(entries)}
}

func printAlertHuman(entries []port.PortEntry) error {
	fmt.Printf("\nALERT: %d new port listener(s) detected!\n\n", len(entries))

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PORT\tPROTO\tPID\tPROCESS\tUSER\tSTATE")
	for _, e := range entries {
		fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s\n",
			e.Port, e.Protocol, e.PID, e.Process, e.User, e.State)
	}
	w.Flush()

	return &alertExitError{count: len(entries)}
}

func watchOnce(ctx context.Context, scanner *port.LsofScanner) error {
	var entries []port.PortEntry
	var err error

	if listAll {
		entries, err = scanner.ListAllPorts(ctx)
	} else {
		entries, err = scanner.ListPorts(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to scan ports: %w", err)
	}

	entries = filterEntries(entries)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Port < entries[j].Port
	})

	// Clear screen.
	fmt.Print("\033[2J\033[H")

	// Header.
	listenCount := 0
	for _, e := range entries {
		if e.State == "LISTEN" {
			listenCount++
		}
	}
	fmt.Printf("whport watch | Listening: %d  Total: %d | %s | Ctrl+C to stop\n\n",
		listenCount, len(entries), time.Now().Format("15:04:05"))

	if len(entries) == 0 {
		fmt.Println("No ports found matching filter.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PORT\tPROTO\tPID\tPROCESS\tUSER\tSTATE\tCOMMAND")
	for _, e := range entries {
		cmd := e.Command
		if len(cmd) > 40 {
			cmd = cmd[:37] + "..."
		}
		fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s\t%s\n",
			e.Port, e.Protocol, e.PID, e.Process, e.User, e.State, cmd)
	}
	w.Flush()

	if filter := activeFilter(); filter != "" {
		fmt.Printf("\nFilter: %s\n", filter)
	}

	return nil
}

func activeFilter() string {
	var parts []string
	if filterPort > 0 {
		parts = append(parts, fmt.Sprintf("port=%d", filterPort))
	}
	if filterProc != "" {
		parts = append(parts, fmt.Sprintf("process=%s", filterProc))
	}
	if filterProto != "" {
		parts = append(parts, fmt.Sprintf("protocol=%s", filterProto))
	}
	return strings.Join(parts, ", ")
}
