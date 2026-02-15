package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/zhengda-lu/whport/internal/port"
)

var watchInterval int

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Auto-refresh port table in terminal",
	Long:  "Continuously display listening ports with periodic refresh.",
	RunE:  runWatch,
}

func init() {
	watchCmd.Flags().IntVar(&watchInterval, "interval", 2, "Refresh interval in seconds")
	watchCmd.Flags().IntVar(&filterPort, "port", 0, "Filter by port number")
	watchCmd.Flags().StringVar(&filterProc, "process", "", "Filter by process name")
	watchCmd.Flags().StringVar(&filterProto, "protocol", "", "Filter by protocol (tcp/udp)")
}

func runWatch(cmd *cobra.Command, args []string) error {
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
