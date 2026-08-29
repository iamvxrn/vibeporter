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
	fmt.Printf("Diff %s → %s  %q\n", from, to, orig.Title)
	fmt.Printf("  Source ID: %s  Messages: %d\n", orig.ID, len(orig.Messages))
	fmt.Printf("  Round-tripped ID: %s  Messages: %d\n", round.ID, len(round.Messages))
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
	fmt.Println("Parts:")
	fmt.Printf("  %-12s  original  round-tripped  diff\n", "kind")
	for _, k := range kinds {
		o := origCounts[k]
		r := roundCounts[k]
		diff := r - o
		sign := ""
		if diff > 0 {
			sign = "+"
		}
		if o == 0 && r == 0 {
			continue
		}
		fmt.Printf("  %-12s  %8d  %13d  %s%d\n", k, o, r, sign, diff)
	}
	fmt.Println()

	lostThinking := origCounts[models.PartThinking] - roundCounts[models.PartThinking]
	lostTools := (origCounts[models.PartToolCall] + origCounts[models.PartToolResult]) - (roundCounts[models.PartToolCall] + roundCounts[models.PartToolResult])
	if lostThinking > 0 {
		fmt.Printf("  ⚠ %d thinking block(s) would be dropped (target does not store reasoning as thinking)\n", lostThinking)
	}
	if lostTools > 0 {
		fmt.Printf("  ⚠ %d tool call/result part(s) would be dropped or merged\n", lostTools)
	}
	if len(orig.Messages) != len(round.Messages) {
		fmt.Printf("  ⚠ Message count changes %d → %d (consecutive same-role merges or empty messages dropped)\n", len(orig.Messages), len(round.Messages))
	}
	if lostThinking == 0 && lostTools == 0 && len(orig.Messages) == len(round.Messages) {
		fmt.Println("  ✓ No visible loss — round-trip preserves text, thinking, and tools for this chat.")
	}

	if len(orig.Messages) > 0 && len(round.Messages) > 0 {
		fmt.Println()
		fmt.Println("Preview (first 2 messages, original → round-tripped):")
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
				fmt.Printf("  [%d %s] identical (%d chars)\n", i, orig.Messages[i].Role, len(o))
			} else {
				fmt.Printf("  [%d %s] differs: orig %d chars → round %d chars\n", i, orig.Messages[i].Role, len(o), len(r))
			}
		}
	}

	fmt.Println()
	fmt.Printf("Tip: run `vibeporter export --from %s --source %s --format markdown | less` to inspect the full text before migrating.\n", from, orig.ID)
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
