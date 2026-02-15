package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/lu-zhengda/whport/internal/port"
	"github.com/lu-zhengda/whport/internal/process"
)

var (
	forceKill  bool
	signalFlag string
)

var killCmd = &cobra.Command{
	Use:   "kill <port>",
	Short: "Kill process listening on a port",
	Long:  "Send a signal to the process listening on the specified port.",
	Args:  cobra.ExactArgs(1),
	RunE:  runKill,
}

func init() {
	killCmd.Flags().BoolVar(&forceKill, "force", false, "Send SIGKILL instead of SIGTERM")
	killCmd.Flags().StringVar(&signalFlag, "signal", "", "Custom signal to send (e.g. SIGINT, SIGHUP)")
}

func runKill(cmd *cobra.Command, args []string) error {
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

	// Filter to LISTEN entries.
	var listeners []port.PortEntry
	for _, e := range entries {
		if e.State == "LISTEN" && e.Port == portNum {
			listeners = append(listeners, e)
		}
	}

	if len(listeners) == 0 {
		return fmt.Errorf("no process listening on port %d", portNum)
	}

	// Kill all listeners on the port (usually just one process).
	for _, e := range listeners {
		sig := resolveSignal()

		// Verify the process is what we expect.
		if !manager.VerifyProcess(ctx, e.PID, e.Process) {
			fmt.Printf("Warning: PID %d may have changed since scan, skipping.\n", e.PID)
			continue
		}

		fmt.Printf("Killing %s (PID %d) on port %d with %s...\n",
			e.Process, e.PID, e.Port, signalName(sig))

		if forceKill || sig == syscall.SIGKILL {
			if err := manager.ForceKill(e.PID); err != nil {
				return fmt.Errorf("failed to kill PID %d: %w", e.PID, err)
			}
			fmt.Printf("Sent SIGKILL to PID %d.\n", e.PID)
		} else if signalFlag != "" {
			if err := manager.Kill(e.PID, sig); err != nil {
				return fmt.Errorf("failed to send signal to PID %d: %w", e.PID, err)
			}
			fmt.Printf("Sent %s to PID %d.\n", signalName(sig), e.PID)
		} else {
			exited, err := manager.GracefulKill(e.PID)
			if err != nil {
				return fmt.Errorf("failed to kill PID %d: %w", e.PID, err)
			}
			if exited {
				fmt.Printf("Process %s (PID %d) terminated gracefully.\n", e.Process, e.PID)
			} else {
				fmt.Printf("Process %s (PID %d) did not exit after SIGTERM.\n", e.Process, e.PID)
				fmt.Println("Use --force to send SIGKILL.")
			}
		}
	}

	return nil
}

func resolveSignal() syscall.Signal {
	if forceKill {
		return syscall.SIGKILL
	}
	if signalFlag != "" {
		return parseSignal(signalFlag)
	}
	return syscall.SIGTERM
}

func parseSignal(s string) syscall.Signal {
	s = strings.ToUpper(strings.TrimPrefix(s, "SIG"))
	switch s {
	case "KILL", "SIGKILL":
		return syscall.SIGKILL
	case "TERM", "SIGTERM":
		return syscall.SIGTERM
	case "INT", "SIGINT":
		return syscall.SIGINT
	case "HUP", "SIGHUP":
		return syscall.SIGHUP
	case "USR1", "SIGUSR1":
		return syscall.SIGUSR1
	case "USR2", "SIGUSR2":
		return syscall.SIGUSR2
	default:
		return syscall.SIGTERM
	}
}

func signalName(sig syscall.Signal) string {
	switch sig {
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGKILL:
		return "SIGKILL"
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGHUP:
		return "SIGHUP"
	case syscall.SIGUSR1:
		return "SIGUSR1"
	case syscall.SIGUSR2:
		return "SIGUSR2"
	default:
		return fmt.Sprintf("signal(%d)", sig)
	}
}
