package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/lu-zhengda/whport/internal/history"
	"github.com/lu-zhengda/whport/internal/port"
	"github.com/spf13/cobra"
)

var historyLimit int

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show port open/close event history",
	Long: `Display a timeline of port open and close events.

Events are recorded each time "whport history record" is run.
The history log is stored at ~/.config/whport/history.json.`,
	RunE: runHistoryShow,
}

var historyRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Snapshot current ports and record changes",
	Long: `Take a snapshot of current listening ports, diff against the
previous snapshot, and record any open/close events.`,
	RunE: runHistoryRecord,
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all history data",
	RunE:  runHistoryClear,
}

func init() {
	historyCmd.Flags().IntVarP(&historyLimit, "last", "n", 0, "Show only the last N events")
	historyCmd.AddCommand(historyRecordCmd)
	historyCmd.AddCommand(historyClearCmd)
}

func runHistoryShow(cmd *cobra.Command, args []string) error {
	store, err := history.NewStore()
	if err != nil {
		return fmt.Errorf("failed to create history store: %w", err)
	}

	data, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	events := data.Events
	if len(events) == 0 {
		if jsonOutput {
			return printHistoryJSON(events)
		}
		fmt.Println("No history events recorded.")
		fmt.Println("Run 'whport history record' to start tracking port changes.")
		return nil
	}

	// Sort by timestamp descending (most recent first).
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	if historyLimit > 0 && historyLimit < len(events) {
		events = events[:historyLimit]
	}

	if jsonOutput {
		return printHistoryJSON(events)
	}

	return printHistoryHuman(events)
}

func runHistoryRecord(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	runner := &port.RealCmdRunner{}
	scanner := port.NewLsofScanner(runner)

	entries, err := scanner.ListPorts(ctx)
	if err != nil {
		return fmt.Errorf("failed to scan ports: %w", err)
	}

	store, err := history.NewStore()
	if err != nil {
		return fmt.Errorf("failed to create history store: %w", err)
	}

	now := time.Now()
	events, err := store.Record(entries, now)
	if err != nil {
		return fmt.Errorf("failed to record history: %w", err)
	}

	if jsonOutput {
		return printHistoryJSON(events)
	}

	if len(events) == 0 {
		fmt.Printf("Snapshot recorded at %s. No changes detected.\n", now.Format("15:04:05"))
		return nil
	}

	fmt.Printf("Snapshot recorded at %s. %d change(s):\n\n", now.Format("15:04:05"), len(events))
	return printHistoryHuman(events)
}

func runHistoryClear(cmd *cobra.Command, args []string) error {
	store, err := history.NewStore()
	if err != nil {
		return fmt.Errorf("failed to create history store: %w", err)
	}

	if err := store.Save(&history.Data{}); err != nil {
		return fmt.Errorf("failed to clear history: %w", err)
	}

	fmt.Println("History cleared.")
	return nil
}

func printHistoryHuman(events []history.Event) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tEVENT\tPORT\tPROTO\tPID\tPROCESS\tUSER")
	for _, e := range events {
		eventStr := "OPEN"
		if e.Type == history.EventClose {
			eventStr = "CLOSE"
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%d\t%s\t%s\n",
			e.Timestamp.Format("2006-01-02 15:04:05"),
			eventStr,
			e.Port,
			e.Protocol,
			e.PID,
			e.Process,
			e.User,
		)
	}
	return w.Flush()
}

func printHistoryJSON(events []history.Event) error {
	type jsonEvent struct {
		Timestamp string `json:"timestamp"`
		Type      string `json:"type"`
		Port      int    `json:"port"`
		Protocol  string `json:"protocol"`
		PID       int    `json:"pid"`
		Process   string `json:"process"`
		User      string `json:"user"`
	}

	out := make([]jsonEvent, len(events))
	for i, e := range events {
		out[i] = jsonEvent{
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Type:      string(e.Type),
			Port:      e.Port,
			Protocol:  e.Protocol,
			PID:       e.PID,
			Process:   e.Process,
			User:      e.User,
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
