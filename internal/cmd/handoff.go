package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"vibeporter/internal/compact"
	"vibeporter/internal/handoff"
)

var (
	handoffFrom, handoffTo, handoffSource, handoffTarget, handoffBudget, handoffStrategy string
	handoffDryRun, handoffJSON                                                           bool
)

var handoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Deliver a context packet into a native session in another agent",
	Long: `Select local engineering context to a token budget and create a fresh native
session in the target agent. The result is a context packet: compacted source
session plus provenance. It never overwrites the source and uses no cloud or LLM.

Examples:
  vibeporter handoff --from claudecode --source abc123 --to opencode --compact 200k
  vibeporter handoff --from cursor --source /path/to/chat --to gemini --compact 100k --strategy recent --dry-run`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		budget, err := compact.ParseBudget(handoffBudget)
		if err != nil {
			return err
		}
		strategy := compact.Strategy(strings.ToLower(strings.TrimSpace(handoffStrategy)))
		extractor, ok := extractors[strings.ToLower(handoffFrom)]
		if !ok {
			return fmt.Errorf("source agent %s is not supported (have: %s)", handoffFrom, supportedExtractors)
		}
		injector, ok := injectors[strings.ToLower(handoffTo)]
		if !ok {
			return fmt.Errorf("target agent %s is not supported (have: %s)", handoffTo, supportedInjectors)
		}
		resolved, err := resolveSource(extractor, handoffSource)
		if err != nil {
			return fmt.Errorf("resolving source: %w", err)
		}
		conversation, err := extractor.Extract(resolved)
		if err != nil {
			return fmt.Errorf("extracting: %w", err)
		}
		result, err := handoff.Execute(conversation, injector, handoff.Options{SourceAgent: handoffFrom, SourceID: conversation.ID, TargetAgent: handoffTo, TargetPath: handoffTarget, Budget: budget, Strategy: strategy, DryRun: handoffDryRun})
		if err != nil {
			return err
		}
		if handoffJSON {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		}
		if handoffDryRun {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Dry run: %s -> %s, tokens~ %d -> %d (budget %d)\n", result.SourceAgent, result.TargetAgent, result.Original, result.Transferred, result.Budget)
			return nil
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created context packet %s -> %s: tokens~ %d -> %d\nWrote %s\n", result.SourceAgent, result.TargetAgent, result.Original, result.Transferred, result.TargetPath)
		return nil
	},
}

func init() {
	handoffCmd.Flags().StringVar(&handoffFrom, "from", "", "Source agent")
	handoffCmd.Flags().StringVar(&handoffTo, "to", "", "Target agent")
	handoffCmd.Flags().StringVar(&handoffSource, "source", "", "Source session id from list, or a file path")
	handoffCmd.Flags().StringVar(&handoffTarget, "target", "", "Optional output path (defaults to the target agent's native store)")
	handoffCmd.Flags().StringVar(&handoffBudget, "compact", "", "Required context budget: 50k, 100k, 200k, or tokens")
	handoffCmd.Flags().StringVar(&handoffStrategy, "strategy", "smart", "Compaction strategy: smart or recent")
	handoffCmd.Flags().BoolVar(&handoffDryRun, "dry-run", false, "Preview without creating a session")
	handoffCmd.Flags().BoolVar(&handoffJSON, "json", false, "Output structured JSON")
	_ = handoffCmd.MarkFlagRequired("from")
	_ = handoffCmd.MarkFlagRequired("to")
	_ = handoffCmd.MarkFlagRequired("source")
	_ = handoffCmd.MarkFlagRequired("compact")
	rootCmd.AddCommand(handoffCmd)
}
