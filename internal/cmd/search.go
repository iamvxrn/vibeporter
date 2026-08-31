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
	"vibeporter/internal/models"
)

var (
	searchAgent string
	searchLimit int
	searchJSON  bool
)

type searchHit struct {
	Agent     string `json:"agent"`
	ID        string `json:"id"`
	Title     string `json:"title"`
	Project   string `json:"project"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Path      string `json:"path"`
	Snippet   string `json:"snippet"`
	Matches   int    `json:"matches"`
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across chats of all agents",
	Long: `Search for a phrase across all chats of all agents (or a single --agent).

Example:
  vibeporter search "fix database bug"
  vibeporter search "auth" --agent gemini --limit 20
  vibeporter search "panic" --json`,
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		query = strings.TrimSpace(query)
		if query == "" {
			return fmt.Errorf("query cannot be empty")
		}
		agents := targetAgents(searchAgent)
		results, err := runSearch(query, agents, searchLimit)
		if err != nil {
			return err
		}
		if searchJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if results == nil {
				results = []searchHit{}
			}
			return enc.Encode(results)
		}
		printSearchHuman(query, results)
		return nil
	},
}

func targetAgents(filter string) []string {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter != "" {
		if _, ok := extractors[filter]; ok {
			return []string{filter}
		}
		return []string{filter}
	}
	// all extractor keys deduped and sorted
	seen := map[string]bool{}
	var out []string
	for k := range extractors {
		// normalize aliases: kimi/kimicode, dsh/dhs
		if k == "kimi" || k == "dhs" {
			continue
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func runSearch(query string, agents []string, limit int) ([]searchHit, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	qLower := strings.ToLower(query)
	var hits []searchHit

	for _, agent := range agents {
		extractor, ok := extractors[agent]
		if !ok {
			return nil, fmt.Errorf("agent %s is not supported (have: %s)", agent, supportedExtractors)
		}
		chats, err := extractor.ListConversations()
		if err != nil {
			// e.g. no data dir — skip
			continue
		}
		for _, c := range chats {
			if len(hits) >= limit {
				break
			}
			conv, err := extractor.Extract(c.Path)
			if err != nil {
				continue
			}
			matches, snippet := matchConversation(conv, qLower, query)
			if matches == 0 {
				// also match title/project without extracting snippet from them
				if strings.Contains(strings.ToLower(c.Title), qLower) || strings.Contains(strings.ToLower(c.Project), qLower) {
					matches = 1
					if snippet == "" {
						snippet = clipSnippet(c.Title, query, 80)
					}
				} else {
					continue
				}
			}
			updated := ""
			if !c.UpdatedAt.IsZero() {
				updated = c.UpdatedAt.UTC().Format(time.RFC3339)
			} else if conv != nil && len(conv.Messages) > 0 && conv.Messages[0].Timestamp != nil {
				updated = conv.Messages[0].Timestamp.UTC().Format(time.RFC3339)
			}
			hits = append(hits, searchHit{
				Agent:     agent,
				ID:        c.ID,
				Title:     c.Title,
				Project:   c.Project,
				Path:      c.Path,
				UpdatedAt: updated,
				Snippet:   snippet,
				Matches:   matches,
			})
		}
		if len(hits) >= limit {
			break
		}
	}

	// Sort by UpdatedAt desc (newest first), fallback to title
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].UpdatedAt != hits[j].UpdatedAt {
			return hits[i].UpdatedAt > hits[j].UpdatedAt
		}
		return hits[i].Title < hits[j].Title
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func matchConversation(conv *models.Conversation, qLower, qOrig string) (int, string) {
	if conv == nil {
		return 0, ""
	}
	matches := 0
	var firstSnippet string
	// check title first
	if strings.Contains(strings.ToLower(conv.Title), qLower) {
		matches++
		firstSnippet = clipSnippet(conv.Title, qOrig, 80)
	}
	for _, m := range conv.Messages {
		// Content fallback
		if strings.Contains(strings.ToLower(m.Content), qLower) {
			matches++
			if firstSnippet == "" {
				firstSnippet = clipSnippet(m.Content, qOrig, 120)
			}
		}
		for _, p := range m.EffectiveParts() {
			text := ""
			switch p.Kind {
			case models.PartText, models.PartThinking, models.PartToolResult:
				text = p.Text
			case models.PartToolCall:
				text = p.Name + " " + p.ArgsJSON + " " + p.Text
			}
			if strings.Contains(strings.ToLower(text), qLower) {
				matches++
				if firstSnippet == "" {
					src := text
					if p.Kind == models.PartThinking {
						// keep thinking snippet a bit shorter
						if len(src) > 120 {
							src = src[:120]
						}
					}
					firstSnippet = clipSnippet(src, qOrig, 120)
				}
			}
			// also match args/json separately
			if p.ArgsJSON != "" && strings.Contains(strings.ToLower(p.ArgsJSON), qLower) && firstSnippet == "" {
				matches++
				firstSnippet = clipSnippet(p.ArgsJSON, qOrig, 120)
			}
		}
	}
	return matches, firstSnippet
}

func clipSnippet(text, query string, maxLen int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if text == "" {
		return ""
	}
	runes := []rune(text)
	lower := []rune(strings.ToLower(text))
	qLower := []rune(strings.ToLower(query))
	idx := runeIndex(lower, qLower)
	if idx < 0 {
		if len(runes) > maxLen {
			return adapters.Clip(string(runes), maxLen)
		}
		return text
	}
	// center snippet around match
	start := idx - maxLen/3
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(runes) {
		end = len(runes)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}
	snip := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snip = "…" + snip
	}
	if end < len(runes) {
		snip = snip + "…"
	}
	// highlight query case-insensitively by uppercasing matched slice (simple)
	// Keep original casing but ensure snippet readable
	return snip
}

func runeIndex(text, query []rune) int {
	if len(query) == 0 {
		return -1
	}
	for i := 0; i+len(query) <= len(text); i++ {
		if string(text[i:i+len(query)]) == string(query) {
			return i
		}
	}
	return -1
}

func printSearchHuman(query string, hits []searchHit) {
	if len(hits) == 0 {
		fmt.Printf("%s No matches for %s.\n", colorize(colorDim, "○"), colorize(colorBrightYellow+colorBold, fmt.Sprintf("%q", query)))
		fmt.Printf("  %s\n", colorize(colorDim, "Try a broader query or check --agent filter."))
		return
	}
	fmt.Printf("%s %s for %s\n\n", colorize(colorBrightMagenta+colorBold, "🔍"), colorize(colorBrightGreen+colorBold, fmt.Sprintf("%d match(es)", len(hits))), colorize(colorBrightYellow+colorBold, fmt.Sprintf("%q", query)))
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, colorize(colorBrightCyan+colorBold+colorUnderline, "AGENT")+"\t"+colorize(colorDim, "TITLE")+"\t"+colorize(colorDim, "PROJECT")+"\t"+colorize(colorDim, "UPDATED")+"\t"+colorize(colorDim, "MATCHES")+"\t"+colorize(colorDim, "ID"))
	_, _ = fmt.Fprintln(w, colorize(colorDim, strings.Repeat("─", 8))+"\t"+colorize(colorDim, strings.Repeat("─", 20))+"\t"+colorize(colorDim, strings.Repeat("─", 12))+"\t"+colorize(colorDim, strings.Repeat("─", 10))+"\t"+colorize(colorDim, strings.Repeat("─", 7))+"\t"+colorize(colorDim, strings.Repeat("─", 8)))
	for i, h := range hits {
		title := h.Title
		if title == "" {
			title = "Untitled"
		}
		title = adapters.Clip(strings.ReplaceAll(title, "\t", " "), 40)
		title = highlightMatch(title, query)
		if i%2 == 1 {
			title = colorize(colorDim, title)
		} else {
			title = colorize(colorBold, title)
		}
		updated := ""
		if h.UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339, h.UpdatedAt); err == nil {
				updated = colorize(colorDim, t.Local().Format("2006-01-02"))
			} else {
				updated = colorize(colorDim, h.UpdatedAt)
			}
		}
		agentDisp := colorize(agentColor(h.Agent)+colorBold, h.Agent) + " " + colorize(colorDim, agentIcon(h.Agent))
		matchesDisp := colorize(colorBrightYellow+colorBold, fmt.Sprintf("%d", h.Matches))
		idDisp := colorize(colorCyan, h.ID)
		projectDisp := colorize(colorDim, h.Project)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", agentDisp, title, projectDisp, updated, matchesDisp, idDisp)
	}
	_ = w.Flush()
	fmt.Println()
	for _, h := range hits {
		if h.Snippet != "" {
			snip := highlightMatch(h.Snippet, query)
			idShort := h.ID
			if len(idShort) > 8 {
				idShort = idShort[:8]
			}
			fmt.Printf("  %s %s — %s\n", colorize(agentColor(h.Agent), "▸"), colorize(agentColor(h.Agent)+colorBold, h.Agent+"/"+idShort), colorize(colorDim, snip))
		}
	}
	fmt.Printf("\n%s %s\n", colorize(colorDim, "─"), colorize(colorDim, fmt.Sprintf("%d hit(s) — use --json for scripts or --agent to filter", len(hits))))
}

func init() {
	searchCmd.Flags().StringVar(&searchAgent, "agent", "", "Only search this agent (e.g. gemini, claudecode, opencode, cursor)")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Max results")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "Machine-readable JSON")
	rootCmd.AddCommand(searchCmd)
}
