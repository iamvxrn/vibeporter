package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

var (
	statsAgent string
	statsJSON  bool
)

type agentStats struct {
	Agent      string `json:"agent"`
	Chats      int    `json:"chats"`
	Messages   int    `json:"messages"`
	TextParts  int    `json:"text_parts"`
	Thinking   int    `json:"thinking"`
	ToolCalls  int    `json:"tool_calls"`
	ToolResult int    `json:"tool_results"`
	Chars      int    `json:"chars"`
	TokensEst  int    `json:"tokens_est"`
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show analytics per agent — chats, messages, tokens, tool calls",
	Long: `Aggregate stats across all agents' chats.

Example:
  vibeporter stats
  vibeporter stats --agent gemini --json`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		agents := targetAgents(statsAgent)
		// for stats we need to validate agent if filtered
		if statsAgent != "" {
			if _, ok := extractors[statsAgent]; !ok {
				return fmt.Errorf("agent %s is not supported (have: %s)", statsAgent, supportedExtractors)
			}
		}
		rows, err := collectStats(agents)
		if err != nil {
			return err
		}
		if statsJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if rows == nil {
				rows = []agentStats{}
			}
			return enc.Encode(rows)
		}
		printStatsHuman(rows)
		return nil
	},
}

func collectStats(agents []string) ([]agentStats, error) {
	var out []agentStats
	for _, agent := range agents {
		extractor, ok := extractors[agent]
		if !ok {
			continue
		}
		chats, err := extractor.ListConversations()
		if err != nil {
			continue
		}
		row := agentStats{Agent: agent, Chats: len(chats)}
		for _, c := range chats {
			conv, err := extractor.Extract(c.Path)
			if err != nil || conv == nil {
				continue
			}
			row.Messages += len(conv.Messages)
			for _, m := range conv.Messages {
				// Count chars from content and parts
				row.Chars += len(m.Content)
				for _, p := range m.EffectiveParts() {
					switch p.Kind {
					case models.PartText:
						row.TextParts++
						row.Chars += len(p.Text)
					case models.PartThinking:
						row.Thinking++
						row.Chars += len(p.Text)
					case models.PartToolCall:
						row.ToolCalls++
						row.Chars += len(p.Name) + len(p.ArgsJSON)
					case models.PartToolResult:
						row.ToolResult++
						row.Chars += len(p.Text)
					}
				}
			}
		}
		// Estimate tokens: ~4 chars per token (common heuristic)
		if row.Chars > 0 {
			row.TokensEst = (row.Chars + 3) / 4
		}
		// Also include title chars already counted? Messages cover content, but okay
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Agent < out[j].Agent })
	return out, nil
}

func printStatsHuman(rows []agentStats) {
	if len(rows) == 0 {
		fmt.Println("No chats found for any agent.")
		return
	}
	// totals
	var total agentStats
	total.Agent = "TOTAL"
	for _, r := range rows {
		total.Chats += r.Chats
		total.Messages += r.Messages
		total.TextParts += r.TextParts
		total.Thinking += r.Thinking
		total.ToolCalls += r.ToolCalls
		total.ToolResult += r.ToolResult
		total.Chars += r.Chars
		total.TokensEst += r.TokensEst
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "AGENT\tCHATS\tMSGS\tTEXT\tTHINK\tTOOLS\tCHARS\tTOKENS~")
	for _, r := range rows {
		tools := r.ToolCalls + r.ToolResult
		_, _ = fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n", r.Agent, r.Chats, r.Messages, r.TextParts, r.Thinking, tools, r.Chars, r.TokensEst)
	}
	toolsTotal := total.ToolCalls + total.ToolResult
	_, _ = fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n", total.Agent, total.Chats, total.Messages, total.TextParts, total.Thinking, toolsTotal, total.Chars, total.TokensEst)
	_ = w.Flush()

	// simple bar graph for chats distribution
	fmt.Println()
	fmt.Println("Chats per agent:")
	maxChats := 0
	for _, r := range rows {
		if r.Chats > maxChats {
			maxChats = r.Chats
		}
	}
	if maxChats > 0 {
		for _, r := range rows {
			barLen := 0
			if maxChats > 0 {
				barLen = (r.Chats * 20) / maxChats
				if r.Chats > 0 && barLen == 0 {
					barLen = 1
				}
			}
			bar := strings.Repeat("█", barLen)
			fmt.Printf("  %-12s %4d %s\n", r.Agent, r.Chats, bar)
		}
	}
	fmt.Println()
	fmt.Printf("Tip: vibeporter search \"query\" --agent gemini  |  vibeporter stats --json | jq\n")
	// avoid unused import for adapters
	_ = adapters.Clip
}

func init() {
	statsCmd.Flags().StringVar(&statsAgent, "agent", "", "Only this agent (e.g. gemini, claudecode)")
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Machine-readable JSON")
	rootCmd.AddCommand(statsCmd)
}
