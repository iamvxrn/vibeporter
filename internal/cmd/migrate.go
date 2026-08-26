package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	fromAgent  string
	toAgent    string
	sourcePath string
	targetPath string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate a chat from one agent format to another",
	Run: func(cmd *cobra.Command, args []string) {
		extractor, ok := extractors[fromAgent]
		if !ok {
			fmt.Printf("Source agent %s not supported\n", fromAgent)
			return
		}

		injector, ok := injectors[toAgent]
		if !ok {
			fmt.Printf("Target agent %s not supported\n", toAgent)
			return
		}

		fmt.Printf("Migrating chat from %s (%s) to %s (%s)...\n", sourcePath, fromAgent, targetPath, toAgent)

		resolved, err := resolveSource(extractor, sourcePath)
		if err != nil {
			fmt.Printf("Error resolving source: %v\n", err)
			return
		}

		conv, err := extractor.Extract(resolved)
		if err != nil {
			fmt.Printf("Error extracting: %v\n", err)
			return
		}

		outPath, err := injector.Inject(conv, targetPath)
		if err != nil {
			fmt.Printf("Error injecting: %v\n", err)
			return
		}

		fmt.Printf("Success! Migrated %d messages to %s.\n", len(conv.Messages), outPath)
	},
}

func init() {
	migrateCmd.Flags().StringVar(&fromAgent, "from", "", "Source agent (e.g. claudecode)")
	migrateCmd.Flags().StringVar(&toAgent, "to", "", "Target agent (e.g. gemini)")
	migrateCmd.Flags().StringVar(&sourcePath, "source", "", "Chat id from `list`, or a file path")
	migrateCmd.Flags().StringVar(&targetPath, "target", "", "Path to write the target chat log")

	migrateCmd.MarkFlagRequired("from")
	migrateCmd.MarkFlagRequired("to")
	migrateCmd.MarkFlagRequired("source")
	migrateCmd.MarkFlagRequired("target")

	rootCmd.AddCommand(migrateCmd)
}
