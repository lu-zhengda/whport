package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zhengda-lu/whport/internal/port"
)

var (
	listAll      bool
	filterPort   int
	filterProc   string
	filterProto  string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all listening ports",
	Long:  "Display a table of all ports currently in use by processes.",
	RunE:  runList,
}

func init() {
	listCmd.Flags().BoolVar(&listAll, "all", false, "Include ESTABLISHED connections (not just LISTEN)")
	listCmd.Flags().IntVar(&filterPort, "port", 0, "Filter by port number")
	listCmd.Flags().StringVar(&filterProc, "process", "", "Filter by process name")
	listCmd.Flags().StringVar(&filterProto, "protocol", "", "Filter by protocol (tcp/udp)")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	runner := &port.RealCmdRunner{}
	scanner := port.NewLsofScanner(runner)

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

	// Sort by port number.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Port < entries[j].Port
	})

	if jsonOutput {
		return printJSON(entries)
	}

	return printTable(entries)
}

func filterEntries(entries []port.PortEntry) []port.PortEntry {
	var filtered []port.PortEntry
	for _, e := range entries {
		if filterPort > 0 && e.Port != filterPort {
			continue
		}
		if filterProc != "" && !strings.Contains(strings.ToLower(e.Process), strings.ToLower(filterProc)) {
			continue
		}
		if filterProto != "" {
			want := strings.ToUpper(filterProto)
			if string(e.Protocol) != want {
				continue
			}
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func printTable(entries []port.PortEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PORT\tPROTO\tPID\tPROCESS\tUSER\tSTATE")
	for _, e := range entries {
		fmt.Fprintf(w, "%d\t%s\t%d\t%s\t%s\t%s\n",
			e.Port, e.Protocol, e.PID, e.Process, e.User, e.State)
	}
	return w.Flush()
}

func printJSON(entries []port.PortEntry) error {
	type jsonEntry struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		PID      int    `json:"pid"`
		Process  string `json:"process"`
		User     string `json:"user"`
		State    string `json:"state"`
		Command  string `json:"command"`
	}

	out := make([]jsonEntry, len(entries))
	for i, e := range entries {
		out[i] = jsonEntry{
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
	return enc.Encode(out)
}
