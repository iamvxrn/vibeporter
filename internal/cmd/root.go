package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vibeporter",
	Short: "Preserve engineering context locally and make it portable across agents and projects",
	Long: `Vibeporter preserves engineering context locally and makes it portable across AI agents, developers, teams, and projects.

The unit of transfer is a context packet: selected project context with provenance, not a full chat archive. Task handoff, adapters, and config porting are integrations that deliver that packet into another agent or file.

Vibeporter is local-only today: no cloud account, no telemetry, no background service.

Examples:
  vibeporter list claudecode
  vibeporter handoff --from claudecode --source <id> --to opencode --compact 200k
  vibeporter serve`,
	Version:       "0.5.0",
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if handoffJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
			os.Exit(1)
		}
		fmt.Println(err)
		os.Exit(1)
	}
}
