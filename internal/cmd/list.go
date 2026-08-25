package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [agent]",
	Short: "List available chats for a specific agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		agent := args[0]
		
		extractor, ok := extractors[agent]
		if !ok {
			fmt.Printf("Agent %s not supported for extraction\n", agent)
			return
		}

		chats, err := extractor.ListConversations()
		if err != nil {
			fmt.Printf("Error listing chats: %v\n", err)
			return
		}

		fmt.Printf("Found %d chats for %s:\n", len(chats), agent)
		for _, c := range chats {
			fmt.Printf("- %s\n  Path: %s\n", c.ID, c.Path)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
