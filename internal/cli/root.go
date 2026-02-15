package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/zhengda-lu/whport/internal/port"
	"github.com/zhengda-lu/whport/internal/process"
	"github.com/zhengda-lu/whport/internal/tui"
)

var (
	// Set via ldflags at build time.
	version = "dev"

	// Global flags.
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "whport",
	Short: "Port & process manager for macOS",
	Long: `whport shows what processes are listening on which ports,
lets you kill them, and provides a live TUI dashboard.
Launch without subcommands for interactive TUI mode.`,
	Version: version,
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &port.RealCmdRunner{}
		scanner := port.NewLsofScanner(runner)
		manager := process.NewRealManager(runner)

		p := tea.NewProgram(tui.New(scanner, manager, version), tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("whport %s\n", version))
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(killCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(watchCmd)
}
