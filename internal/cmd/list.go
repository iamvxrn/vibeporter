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
			return fmt.Errorf("agent %s is not supported for extraction (have: %s)", agent, supportedExtractors)
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

func agentColor(agent string) string {
	switch agent {
	case "claudecode":
		return colorBrightMagenta
	case "cursor":
		return colorBrightCyan
	case "opencode":
		return colorBrightGreen
	case "antigravity", "ag":
		return colorBrightBlue
	case "gemini":
		return colorBlue
	case "kimicode", "kimi":
		return colorYellow
	case "windsurf", "wind":
		return colorCyan
	default:
		return colorBrightMagenta
	}
}

func agentIcon(agent string) string {
	switch agent {
	case "claudecode":
		return "🤖"
	case "cursor":
		return "⌨️"
	case "opencode":
		return "⚡"
	case "antigravity", "ag":
		return "🌌"
	case "gemini":
		return "✨"
	case "kimicode", "kimi":
		return "🌙"
	case "windsurf":
		return "🏄"
	default:
		return "💬"
	}
}

func printListHuman(agent string, chats []adapters.ChatInfo, showPath bool) {
	acolor := agentColor(agent)
	aicon := agentIcon(agent)
	switch len(chats) {
	case 0:
		fmt.Printf("%s No chats found for %s.\n", colorize(colorDim, "○"), colorize(acolor+colorBold, agent))
		return
	case 1:
		fmt.Printf("%s %s 1 chat  %s\n\n", colorize(acolor, aicon), colorize(colorBold, "▸"), colorize(acolor+colorBold, agent))
	default:
		fmt.Printf("%s %s %d chats  %s\n\n", colorize(acolor, aicon), colorize(colorBold, "▸"), len(chats), colorize(acolor+colorBold, agent))
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	header := colorize(colorBrightCyan+colorBold+colorUnderline, "TITLE") + "\t" + colorize(colorDim, "PROJECT") + "\t" + colorize(colorDim, "UPDATED") + "\t" + colorize(colorDim, "ID")
	if showPath {
		header += "\t" + colorize(colorDim, "PATH")
	}
	_, _ = fmt.Fprintln(w, header)
	// separator
	sep := colorize(colorDim, strings.Repeat("─", 20)) + "\t" + colorize(colorDim, strings.Repeat("─", 12)) + "\t" + colorize(colorDim, strings.Repeat("─", 12)) + "\t" + colorize(colorDim, strings.Repeat("─", 8))
	if showPath {
		sep += "\t" + colorize(colorDim, strings.Repeat("─", 20))
	}
	_, _ = fmt.Fprintln(w, sep)
	for i, c := range chats {
		title := c.Title
		if title == "" {
			title = "Untitled"
		}
		title = adapters.Clip(strings.ReplaceAll(title, "\t", " "), 72)
		// Color title alternately and highlight
		if i%2 == 0 {
			title = colorize(colorBold, title)
		} else {
			title = colorize(colorDim, title)
		}
		updated := ""
		if !c.UpdatedAt.IsZero() {
			updated = colorize(colorDim, c.UpdatedAt.Local().Format("2006-01-02 15:04"))
		}
		id := colorize(colorCyan, c.ID)
		if len(c.ID) > 12 {
			id = colorize(colorCyan, c.ID[:8]+colorize(colorDim, c.ID[8:12]))
		}
		project := colorize(colorDim, c.Project)
		if showPath {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", title, project, updated, id, colorize(colorDim, c.Path))
		} else {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", title, project, updated, id)
		}
	}
	_ = w.Flush()
	if len(chats) > 0 {
		fmt.Printf("\n%s %s\n", colorize(colorDim, "─"), colorize(colorDim, fmt.Sprintf("%d chat(s) — %s %s", len(chats), aicon, agent)))
	}
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
