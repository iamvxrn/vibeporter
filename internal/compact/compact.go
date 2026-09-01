// Package compact reduces conversations locally using a heuristic token estimate.
package compact

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"vibeporter/internal/models"
)

type Strategy string

const (
	Smart  Strategy = "smart"
	Recent Strategy = "recent"
)

type Report struct {
	Strategy Strategy `json:"strategy"`
	Budget   int      `json:"budget_tokens"`
	Original int      `json:"original_tokens_estimate"`
	Result   int      `json:"transferred_tokens_estimate"`
	Kept     int      `json:"messages_kept"`
	Reduced  int      `json:"messages_reduced"`
}

func ParseBudget(value string) (int, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.HasSuffix(value, "k") {
		value = strings.TrimSuffix(value, "k") + "000"
	}
	budget, err := strconv.Atoi(value)
	if err != nil || budget <= 0 {
		return 0, fmt.Errorf("invalid compact budget %q (use e.g. 50k, 100k, 200k, or a positive token count)", value)
	}
	return budget, nil
}

// EstimateTokens is deliberately heuristic: four Unicode code points per token.
func EstimateTokens(conv *models.Conversation) int {
	if conv == nil {
		return 0
	}
	total := 0
	for _, msg := range conv.Messages {
		total += messageTokens(msg)
	}
	return total
}

func messageTokens(msg models.Message) int {
	text := msg.Content
	if len(msg.Parts) > 0 {
		var b strings.Builder
		for _, part := range msg.Parts {
			b.WriteString(part.Text)
			b.WriteString(part.Name)
			b.WriteString(part.ArgsJSON)
		}
		text = b.String()
	}
	return (utf8.RuneCountInString(text) + 3) / 4
}

func Compact(source *models.Conversation, budget int, strategy Strategy) (*models.Conversation, Report, error) {
	if source == nil {
		return nil, Report{}, fmt.Errorf("conversation is required")
	}
	if budget <= 0 {
		return nil, Report{}, fmt.Errorf("compact budget must be positive")
	}
	if strategy == "" {
		strategy = Smart
	}
	if strategy != Smart && strategy != Recent {
		return nil, Report{}, fmt.Errorf("invalid compact strategy %q (use smart or recent)", strategy)
	}
	result := &models.Conversation{ID: source.ID, Title: source.Title, AgentSource: source.AgentSource, Metadata: cloneMeta(source.Metadata)}
	report := Report{Strategy: strategy, Budget: budget, Original: EstimateTokens(source)}
	if report.Original <= budget {
		result.Messages = cloneMessages(source.Messages)
		report.Result, report.Kept = report.Original, len(result.Messages)
		return result, report, nil
	}

	selected := make(map[int]bool)
	used := 0
	add := func(index int) bool {
		if selected[index] || used+messageTokens(source.Messages[index]) > budget {
			return false
		}
		selected[index] = true
		used += messageTokens(source.Messages[index])
		return true
	}
	if strategy == Smart {
		for i, msg := range source.Messages {
			if msg.Role == models.RoleSystem {
				add(i)
			}
		}
		for i, msg := range source.Messages {
			if msg.Role == models.RoleUser {
				add(i)
				break
			}
		}
	}
	for i := len(source.Messages) - 1; i >= 0; i-- {
		add(i)
	}
	for i, msg := range source.Messages {
		if !selected[i] {
			continue
		}
		copy := cloneMessage(msg)
		copy.Parts = validParts(copy.Parts, source.Messages, selected)
		if len(copy.Parts) == 0 && strings.TrimSpace(copy.Content) == "" {
			continue
		}
		copy.SyncContent()
		result.Messages = append(result.Messages, copy)
	}
	if len(result.Messages) == 0 {
		return nil, Report{}, fmt.Errorf("compact budget %d is too small for a valid handoff", budget)
	}
	report.Result, report.Kept = EstimateTokens(result), len(result.Messages)
	if report.Result > budget {
		return nil, Report{}, fmt.Errorf("compact budget %d is too small for the required context", budget)
	}
	report.Reduced = len(source.Messages) - report.Kept
	return result, report, nil
}

func validParts(parts []models.Part, all []models.Message, selected map[int]bool) []models.Part {
	if len(parts) == 0 {
		return nil
	}
	calls := map[string]bool{}
	for i, msg := range all {
		if !selected[i] {
			continue
		}
		for _, part := range msg.EffectiveParts() {
			if part.Kind == models.PartToolCall {
				calls[part.ID] = true
			}
		}
	}
	out := make([]models.Part, 0, len(parts))
	for _, part := range parts {
		if part.Kind == models.PartToolResult && !calls[part.ToolCallID] {
			continue
		}
		out = append(out, part)
	}
	return out
}

func cloneMessages(in []models.Message) []models.Message {
	out := make([]models.Message, len(in))
	for i := range in {
		out[i] = cloneMessage(in[i])
	}
	return out
}
func cloneMessage(in models.Message) models.Message {
	out := in
	out.Parts = append([]models.Part(nil), in.Parts...)
	out.Metadata = cloneMeta(in.Metadata)
	return out
}
func cloneMeta(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
