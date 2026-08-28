package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

var (
	fromAgent    string
	toAgent      string
	sourcePath   string
	targetPath   string
	migrateTitle string
	migrateCwd   string
)

var migrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Migrate a chat from one agent format to another",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		extractor, ok := extractors[fromAgent]
		if !ok {
			return fmt.Errorf("source agent %s is not supported (have: %s)", fromAgent, supportedExtractors)
		}
		injector, ok := injectors[toAgent]
		if !ok {
			return fmt.Errorf("target agent %s is not supported (have: %s)", toAgent, supportedInjectors)
		}

		resolved, err := resolveSource(extractor, sourcePath)
		if err != nil {
			return fmt.Errorf("resolving source: %w", err)
		}

		conv, err := extractor.Extract(resolved)
		if err != nil {
			return fmt.Errorf("extracting: %w", err)
		}
		applyMigrateOverrides(conv)

		out := strings.TrimSpace(targetPath)
		if out == "" {
			if td, ok := injector.(adapters.TargetDefaults); ok {
				out, err = td.DefaultTarget(conv)
				if err != nil {
					return fmt.Errorf("default target: %w", err)
				}
			} else {
				return fmt.Errorf("--target is required for %s", toAgent)
			}
		}

		fmt.Printf("Migrating %d messages from %s to %s...\n", len(conv.Messages), fromAgent, toAgent)
		written, err := injector.Inject(conv, out)
		if err != nil {
			return fmt.Errorf("injecting: %w", err)
		}
		fmt.Printf("Wrote %s\n", written)
		return nil
	},
}

func applyMigrateOverrides(conv *models.Conversation) {
	if conv == nil {
		return
	}
	if t := strings.TrimSpace(migrateTitle); t != "" {
		conv.Title = t
	}
	if cwd := strings.TrimSpace(migrateCwd); cwd != "" {
		adapters.EnsureMeta(conv)["cwd"] = cwd
	}
}

func init() {
	migrateCmd.Flags().StringVar(&fromAgent, "from", "", "Source agent")
	migrateCmd.Flags().StringVar(&toAgent, "to", "", "Target agent")
	migrateCmd.Flags().StringVar(&sourcePath, "source", "", "Chat id from list, or a file path")
	migrateCmd.Flags().StringVar(&targetPath, "target", "", "Optional output path (defaults to the target agent's native store)")
	migrateCmd.Flags().StringVar(&migrateTitle, "title", "", "Override the conversation title on the target")
	migrateCmd.Flags().StringVar(&migrateCwd, "cwd", "", "Override the workspace directory stored on the target session")

	_ = migrateCmd.MarkFlagRequired("from")
	_ = migrateCmd.MarkFlagRequired("to")
	_ = migrateCmd.MarkFlagRequired("source")

	rootCmd.AddCommand(migrateCmd)
}
