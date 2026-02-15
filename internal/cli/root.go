package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lu-zhengda/whport/internal/port"
	"github.com/lu-zhengda/whport/internal/process"
	"github.com/lu-zhengda/whport/internal/tui"
	"github.com/spf13/cobra"
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
		if shell, _ := cmd.Flags().GetString("generate-completion"); shell != "" {
			switch shell {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			default:
				return fmt.Errorf("unsupported shell: %s (use bash, zsh, or fish)", shell)
			}
		}
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
	rootCmd.Flags().String("generate-completion", "", "Generate shell completion (bash, zsh, fish)")
	rootCmd.Flags().MarkHidden("generate-completion")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(killCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(historyCmd)
}
