package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
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
		fmt.Printf("%s No chats found for any agent.\n", colorize(colorDim, "○"))
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

	fmt.Printf("%s %s\n\n", colorize(colorBrightMagenta+colorBold, "📊"), colorize(colorBold, "Agent Analytics"))
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, colorize(colorBrightCyan+colorBold+colorUnderline, "AGENT")+"\t"+colorize(colorBrightYellow+colorBold, "CHATS")+"\t"+colorize(colorDim, "MSGS")+"\t"+colorize(colorDim, "TEXT")+"\t"+colorize(colorDim, "THINK")+"\t"+colorize(colorBrightMagenta+colorBold, "TOOLS")+"\t"+colorize(colorDim, "CHARS")+"\t"+colorize(colorBrightGreen+colorBold, "TOKENS~"))
	_, _ = fmt.Fprintln(w, colorize(colorDim, strings.Repeat("─", 10))+"\t"+colorize(colorDim, strings.Repeat("─", 5))+"\t"+colorize(colorDim, strings.Repeat("─", 6))+"\t"+colorize(colorDim, strings.Repeat("─", 6))+"\t"+colorize(colorDim, strings.Repeat("─", 5))+"\t"+colorize(colorDim, strings.Repeat("─", 6))+"\t"+colorize(colorDim, strings.Repeat("─", 8))+"\t"+colorize(colorDim, strings.Repeat("─", 8)))
	for i, r := range rows {
		tools := r.ToolCalls + r.ToolResult
		agentDisp := colorize(agentColor(r.Agent)+colorBold, r.Agent) + " " + colorize(colorDim, agentIcon(r.Agent))
		chatsDisp := colorize(colorBrightYellow+colorBold, fmt.Sprintf("%d", r.Chats))
		msgsDisp := colorize(colorDim, fmt.Sprintf("%d", r.Messages))
		if r.Messages > 1000 {
			msgsDisp = colorize(colorBrightCyan, fmt.Sprintf("%d", r.Messages))
		}
		toolsDisp := colorize(colorBrightMagenta, fmt.Sprintf("%d", tools))
		if tools > 100 {
			toolsDisp = colorize(colorBrightMagenta+colorBold, fmt.Sprintf("%d", tools))
		}
		tokensDisp := colorize(colorBrightGreen+colorBold, fmt.Sprintf("%d", r.TokensEst))
		// Alternate row dim
		if i%2 == 1 {
			agentDisp = colorize(colorDim, r.Agent)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\t%d\t%s\n", agentDisp, chatsDisp, msgsDisp, r.TextParts, r.Thinking, toolsDisp, r.Chars, tokensDisp)
	}
	toolsTotal := total.ToolCalls + total.ToolResult
	_, _ = fmt.Fprintln(w, colorize(colorDim, strings.Repeat("─", 10))+"\t"+colorize(colorDim, strings.Repeat("─", 5))+"\t"+colorize(colorDim, strings.Repeat("─", 6))+"\t"+colorize(colorDim, strings.Repeat("─", 6))+"\t"+colorize(colorDim, strings.Repeat("─", 5))+"\t"+colorize(colorDim, strings.Repeat("─", 6))+"\t"+colorize(colorDim, strings.Repeat("─", 8))+"\t"+colorize(colorDim, strings.Repeat("─", 8)))
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		colorize(colorBold+colorBrightCyan, "TOTAL"),
		colorize(colorBrightYellow+colorBold, fmt.Sprintf("%d", total.Chats)),
		colorize(colorBold, fmt.Sprintf("%d", total.Messages)),
		colorize(colorDim, fmt.Sprintf("%d", total.TextParts)),
		colorize(colorDim, fmt.Sprintf("%d", total.Thinking)),
		colorize(colorBrightMagenta+colorBold, fmt.Sprintf("%d", toolsTotal)),
		colorize(colorDim, fmt.Sprintf("%d", total.Chars)),
		colorize(colorBrightGreen+colorBold, fmt.Sprintf("%d", total.TokensEst)),
	)
	_ = w.Flush()

	// bar graph for chats distribution with agent colors
	fmt.Println()
	fmt.Printf("%s %s\n", colorize(colorBrightCyan+colorBold, "▇"), colorize(colorBold, "Chats per agent:"))
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
				barLen = (r.Chats * 24) / maxChats
				if r.Chats > 0 && barLen == 0 {
					barLen = 1
				}
			}
			bar := strings.Repeat("█", barLen)
			bar = colorize(agentColor(r.Agent), bar)
			dimBar := ""
			if barLen < 24 {
				dimBar = colorize(colorDim, strings.Repeat("░", 24-barLen))
			}
			fmt.Printf("  %-12s %s %4s %s%s\n",
				colorize(agentColor(r.Agent)+colorBold, r.Agent),
				colorize(agentColor(r.Agent), agentIcon(r.Agent)),
				colorize(colorBrightYellow+colorBold, fmt.Sprintf("%d", r.Chats)),
				bar, dimBar)
		}
	}
	fmt.Println()
	fmt.Printf("%s %s\n", colorize(colorDim, "─"), colorize(colorDim, "Tip: vibeporter search \"query\" --agent gemini  |  vibeporter stats --json | jq"))
}

func init() {
	statsCmd.Flags().StringVar(&statsAgent, "agent", "", "Only this agent (e.g. gemini, claudecode)")
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Machine-readable JSON")
	rootCmd.AddCommand(statsCmd)
}
