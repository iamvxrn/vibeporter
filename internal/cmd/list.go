package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"vibeporter/internal/adapters"
)

var (
	listJSON  bool
	listPaths bool
)

var listCmd = &cobra.Command{
	Use:          "list [agent]",
	Short:        "List chats by title, project, and date",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := args[0]

		extractor, ok := extractors[agent]
		if !ok {
			return fmt.Errorf("agent %s is not supported for extraction (have: %s)", agent, supportedAgents)
		}

		chats, err := extractor.ListConversations()
		if err != nil {
			return fmt.Errorf("listing chats: %w", err)
		}

		sort.SliceStable(chats, func(i, j int) bool {
			ti, tj := chats[i].UpdatedAt, chats[j].UpdatedAt
			if !ti.Equal(tj) {
				return ti.After(tj)
			}
			return chats[i].Title < chats[j].Title
		})

		if listJSON {
			return writeListJSON(chats)
		}
		printListHuman(agent, chats, listPaths)
		return nil
	},
}

func printListHuman(agent string, chats []adapters.ChatInfo, showPath bool) {
	switch len(chats) {
	case 0:
		fmt.Printf("No chats found for %s.\n", agent)
		return
	case 1:
		fmt.Printf("1 chat  %s\n\n", agent)
	default:
		fmt.Printf("%d chats  %s\n\n", len(chats), agent)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if showPath {
		_, _ = fmt.Fprintln(w, "TITLE\tPROJECT\tUPDATED\tID\tPATH")
	} else {
		_, _ = fmt.Fprintln(w, "TITLE\tPROJECT\tUPDATED\tID")
	}
	for _, c := range chats {
		title := c.Title
		if title == "" {
			title = "Untitled"
		}
		title = adapters.Clip(strings.ReplaceAll(title, "\t", " "), 72)
		updated := ""
		if !c.UpdatedAt.IsZero() {
			updated = c.UpdatedAt.Local().Format("2006-01-02 15:04")
		}
		if showPath {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", title, c.Project, updated, c.ID, c.Path)
		} else {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", title, c.Project, updated, c.ID)
		}
	}
	_ = w.Flush()
}

func writeListJSON(chats []adapters.ChatInfo) error {
	type row struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Project string `json:"project,omitempty"`
		Updated string `json:"updated,omitempty"`
		Path    string `json:"path"`
		Agent   string `json:"agent"`
	}
	out := make([]row, 0, len(chats))
	for _, c := range chats {
		item := row{
			ID:      c.ID,
			Title:   c.Title,
			Project: c.Project,
			Path:    c.Path,
			Agent:   c.Agent,
		}
		if !c.UpdatedAt.IsZero() {
			item.Updated = c.UpdatedAt.UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Machine-readable JSON (includes paths)")
	listCmd.Flags().BoolVar(&listPaths, "paths", false, "Include on-disk paths in the table")
	rootCmd.AddCommand(listCmd)
}
