package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

var (
	diffFrom   string
	diffTo     string
	diffSource string
	diffJSON   bool
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show what would be lost or changed by a migration",
	Long: `Compare the original chat with what the target agent would store after migration.

It extracts the source, does a temp migration to the target format, and reports
counts and dropped parts. No real data is written to the target agent's store.

Example:
  vibeporter diff --from claudecode --to gemini --source <id>`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fromExt, ok := extractors[diffFrom]
		if !ok {
			return fmt.Errorf("source agent %s is not supported (have: %s)", diffFrom, supportedExtractors)
		}
		toInj, ok := injectors[diffTo]
		if !ok {
			return fmt.Errorf("target agent %s is not supported (have: %s)", diffTo, supportedInjectors)
		}
		toExt, ok := extractors[diffTo]
		if !ok {
			return fmt.Errorf("target agent %s is not supported for extraction (have: %s)", diffTo, supportedExtractors)
		}

		resolved, err := resolveSource(fromExt, diffSource)
		if err != nil {
			return fmt.Errorf("resolving source: %w", err)
		}
		orig, err := fromExt.Extract(resolved)
		if err != nil {
			return fmt.Errorf("extracting: %w", err)
		}

		tmpDir, err := os.MkdirTemp("", "vibeporter-diff-*")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		var targetPath string
		if diffTo == "opencode" {
			targetPath = filepath.Join(tmpDir, "opencode.db")
		} else {
			targetPath = filepath.Join(tmpDir, "target.jsonl")
		}

		written, err := toInj.Inject(orig, targetPath)
		if err != nil {
			return fmt.Errorf("simulating inject: %w", err)
		}

		var round *models.Conversation
		if diffTo == "opencode" {
			round = simulateOpencodeRoundTrip(orig)
		} else {
			round, err = toExt.Extract(written)
			if err != nil {
				return fmt.Errorf("extracting round-tripped: %w", err)
			}
		}

		if diffJSON {
			return printDiffJSON(orig, round)
		}
		printDiffHuman(orig, round, diffFrom, diffTo)
		return nil
	},
}

// simulateOpencodeRoundTrip mimics what opencode inject stores without touching the real DB.
// It applies the same filtering as opencode.injectSession: drop tool_result, keep
// tool_call even without output, skip empty messages.
func simulateOpencodeRoundTrip(orig *models.Conversation) *models.Conversation {
	if orig == nil {
		return &models.Conversation{}
	}
	round := &models.Conversation{
		ID:          "simulated",
		Title:       orig.Title,
		AgentSource: "opencode",
		Messages:    []models.Message{},
		Metadata:    map[string]interface{}{},
	}
	for _, m := range orig.Messages {
		parts := m.EffectiveParts()
		// Filter like opencode.injectableParts — keep tool calls even without output
		var filtered []models.Part
		for _, p := range parts {
			if p.Kind == models.PartToolResult {
				continue
			}
			filtered = append(filtered, p)
		}
		if len(filtered) == 0 {
			continue
		}
		round.Messages = append(round.Messages, models.NewMessage(m.Role, filtered))
	}
	return round
}

func printDiffHuman(orig, round *models.Conversation, from, to string) {
	fmt.Printf("%s %s %s %s  %s\n",
		colorize(colorBrightMagenta+colorBold, "◈ Diff"),
		colorize(agentColor(from)+colorBold, from),
		colorize(colorDim, "→"),
		colorize(agentColor(to)+colorBold, to),
		colorize(colorDim, fmt.Sprintf("%q", orig.Title)))
	fmt.Printf("  %s %s  %s %d\n", colorize(colorDim, "Source ID:"), colorize(colorCyan, orig.ID), colorize(colorDim, "Messages:"), len(orig.Messages))
	fmt.Printf("  %s %s  %s %d\n", colorize(colorDim, "Round-tripped ID:"), colorize(colorCyan, round.ID), colorize(colorDim, "Messages:"), len(round.Messages))
	fmt.Println()

	counts := func(conv *models.Conversation) map[models.PartKind]int {
		m := map[models.PartKind]int{}
		for _, msg := range conv.Messages {
			for _, p := range msg.EffectiveParts() {
				m[p.Kind]++
			}
			if msg.Role == models.RoleSystem {
				m["system"]++
			}
		}
		return m
	}
	origCounts := counts(orig)
	roundCounts := counts(round)

	kinds := []models.PartKind{models.PartText, models.PartThinking, models.PartToolCall, models.PartToolResult, "system"}
	fmt.Println(colorize(colorBrightCyan+colorBold, "Parts:"))
	fmt.Printf("  %s  %s  %s  %s\n",
		colorize(colorDim+colorUnderline, fmt.Sprintf("%-12s", "kind")),
		colorize(colorDim, "original"),
		colorize(colorDim, "round-tripped"),
		colorize(colorDim, "diff"))
	for _, k := range kinds {
		o := origCounts[k]
		r := roundCounts[k]
		diff := r - o
		sign := ""
		diffColor := colorDim
		if diff > 0 {
			sign = "+"
			diffColor = colorBrightGreen
		} else if diff < 0 {
			diffColor = colorBrightRed
		}
		if o == 0 && r == 0 {
			continue
		}
		kindDisp := colorize(colorBold, fmt.Sprintf("%-12s", k))
		if diff < 0 {
			kindDisp = colorize(colorYellow, fmt.Sprintf("%-12s", k))
		}
		fmt.Printf("  %s  %8s  %13s  %s\n",
			kindDisp,
			colorize(colorDim, fmt.Sprintf("%d", o)),
			colorize(colorBold, fmt.Sprintf("%d", r)),
			colorize(diffColor+colorBold, fmt.Sprintf("%s%d", sign, diff)))
	}
	fmt.Println()

	lostThinking := origCounts[models.PartThinking] - roundCounts[models.PartThinking]
	lostTools := (origCounts[models.PartToolCall] + origCounts[models.PartToolResult]) - (roundCounts[models.PartToolCall] + roundCounts[models.PartToolResult])
	hasLoss := false
	if lostThinking > 0 {
		fmt.Printf("  %s %s\n", colorize(colorBrightYellow+colorBold, "⚠"), colorize(colorBrightYellow, fmt.Sprintf("%d thinking block(s) would be dropped (target does not store reasoning as thinking)", lostThinking)))
		hasLoss = true
	}
	if lostTools > 0 {
		fmt.Printf("  %s %s\n", colorize(colorBrightYellow+colorBold, "⚠"), colorize(colorBrightYellow, fmt.Sprintf("%d tool call/result part(s) would be dropped or merged", lostTools)))
		hasLoss = true
	}
	if len(orig.Messages) != len(round.Messages) {
		fmt.Printf("  %s %s\n", colorize(colorBrightYellow+colorBold, "⚠"), colorize(colorYellow, fmt.Sprintf("Message count changes %d → %d (consecutive same-role merges or empty messages dropped)", len(orig.Messages), len(round.Messages))))
		hasLoss = true
	}
	if !hasLoss {
		fmt.Printf("  %s %s\n", colorize(colorBrightGreen+colorBold, "✔"), colorize(colorBrightGreen, "No visible loss — round-trip preserves text, thinking, and tools for this chat."))
	}

	if len(orig.Messages) > 0 && len(round.Messages) > 0 {
		fmt.Println()
		fmt.Println(colorize(colorDim, "Preview (first 2 messages, original → round-tripped):"))
		n := 2
		if len(orig.Messages) < n {
			n = len(orig.Messages)
		}
		if len(round.Messages) < n {
			n = len(round.Messages)
		}
		for i := 0; i < n; i++ {
			o := orig.Messages[i].StringContent()
			r := round.Messages[i].StringContent()
			if o == r {
				fmt.Printf("  %s [%d %s] %s (%d chars)\n", colorize(colorBrightGreen, "✔"), i, orig.Messages[i].Role, colorize(colorDim, "identical"), len(o))
			} else {
				fmt.Printf("  %s [%d %s] %s: orig %s → round %s\n", colorize(colorBrightYellow, "◐"), i, orig.Messages[i].Role, colorize(colorYellow, "differs"), colorize(colorBrightRed, fmt.Sprintf("%d chars", len(o))), colorize(colorBrightGreen, fmt.Sprintf("%d chars", len(r))))
			}
		}
	}

	fmt.Println()
	fmt.Printf("%s run %s to inspect the full text before migrating.\n",
		colorize(colorDim, "Tip:"),
		colorize(colorBrightCyan+colorUnderline, fmt.Sprintf("vibeporter export --from %s --source %s --format markdown | less", from, orig.ID)))
}

func printDiffJSON(orig, round *models.Conversation) error {
	type counts struct {
		Text       int `json:"text"`
		Thinking   int `json:"thinking"`
		ToolCall   int `json:"tool_call"`
		ToolResult int `json:"tool_result"`
		System     int `json:"system"`
	}
	toCounts := func(conv *models.Conversation) counts {
		var c counts
		for _, m := range conv.Messages {
			for _, p := range m.EffectiveParts() {
				switch p.Kind {
				case models.PartText:
					c.Text++
				case models.PartThinking:
					c.Thinking++
				case models.PartToolCall:
					c.ToolCall++
				case models.PartToolResult:
					c.ToolResult++
				}
			}
			if m.Role == models.RoleSystem {
				c.System++
			}
		}
		return c
	}
	out := map[string]interface{}{
		"from": map[string]interface{}{
			"id":       orig.ID,
			"title":    orig.Title,
			"messages": len(orig.Messages),
			"parts":    toCounts(orig),
		},
		"to": map[string]interface{}{
			"id":       round.ID,
			"title":    round.Title,
			"messages": len(round.Messages),
			"parts":    toCounts(round),
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func init() {
	diffCmd.Flags().StringVar(&diffFrom, "from", "", "Source agent")
	diffCmd.Flags().StringVar(&diffTo, "to", "", "Target agent")
	diffCmd.Flags().StringVar(&diffSource, "source", "", "Chat id from list, or a file path")
	diffCmd.Flags().BoolVar(&diffJSON, "json", false, "Machine-readable JSON")
	_ = diffCmd.MarkFlagRequired("from")
	_ = diffCmd.MarkFlagRequired("to")
	_ = diffCmd.MarkFlagRequired("source")
	rootCmd.AddCommand(diffCmd)
}

var _ = adapters.NewPrefixedID
