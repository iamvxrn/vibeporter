// Package contextpacket is the product unit of shared engineering context.
//
// A packet is not a full chat archive. It is the selected, provenance-tagged
// context that can be delivered into another agent session, exported, or kept
// locally as a handoff manifest. Structured fields (decisions, constraints,
// open questions, artifacts) are part of the schema so later extractors can
// fill them; this package does not invent them from chat text.
package contextpacket

import (
	"strings"
	"time"
	"unicode/utf8"

	"vibeporter/internal/models"
)

const (
	Kind    = "vibeporter.context_packet"
	Version = 1
)

type SourceSession struct {
	Agent string `json:"agent"`
	ID    string `json:"id"`
}

type Delivery struct {
	Kind  string `json:"kind"`
	Agent string `json:"agent,omitempty"`
	Path  string `json:"path,omitempty"`
}

type Provenance struct {
	Tool     string `json:"tool"`
	Strategy string `json:"strategy,omitempty"`
}

type SelectedContext struct {
	Strategy                  string `json:"strategy"`
	BudgetTokens              int    `json:"budget_tokens"`
	OriginalTokensEstimate    int    `json:"original_tokens_estimate"`
	TransferredTokensEstimate int    `json:"transferred_tokens_estimate"`
	MessagesKept              int    `json:"messages_kept"`
	MessagesReduced           int    `json:"messages_reduced"`
}

type Packet struct {
	Kind            string          `json:"kind"`
	Version         int             `json:"version"`
	Title           string          `json:"title,omitempty"`
	Project         string          `json:"project,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	Provenance      Provenance      `json:"provenance"`
	Decisions       []string        `json:"decisions"`
	CurrentState    string          `json:"current_state,omitempty"`
	Constraints     []string        `json:"constraints"`
	OpenQuestions   []string        `json:"open_questions"`
	Artifacts       []string        `json:"artifacts"`
	Handoffs        []Delivery      `json:"handoffs"`
	SourceSessions  []SourceSession `json:"source_sessions"`
	SelectedContext SelectedContext `json:"selected_context"`
}

type Selection struct {
	SourceAgent string
	SourceID    string
	TargetAgent string
	TargetPath  string
	Strategy    string
	Budget      int
	Original    int
	Transferred int
	Kept        int
	Reduced     int
	CreatedAt   time.Time
}

func FromSelection(conv *models.Conversation, sel Selection) Packet {
	created := sel.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	p := Packet{
		Kind:          Kind,
		Version:       Version,
		CreatedAt:     created,
		Provenance:    Provenance{Tool: "vibeporter", Strategy: sel.Strategy},
		Decisions:     []string{},
		Constraints:   []string{},
		OpenQuestions: []string{},
		Artifacts:     []string{},
		Handoffs:      []Delivery{},
		SourceSessions: []SourceSession{{
			Agent: sel.SourceAgent,
			ID:    sel.SourceID,
		}},
		SelectedContext: SelectedContext{
			Strategy:                  sel.Strategy,
			BudgetTokens:              sel.Budget,
			OriginalTokensEstimate:    sel.Original,
			TransferredTokensEstimate: sel.Transferred,
			MessagesKept:              sel.Kept,
			MessagesReduced:           sel.Reduced,
		},
	}
	if conv != nil {
		p.Title = strings.TrimSpace(conv.Title)
		p.Project = projectFrom(conv)
		p.CurrentState = lastUsefulText(conv)
	}
	if strings.TrimSpace(sel.TargetAgent) != "" {
		p.Handoffs = []Delivery{{
			Kind:  "native_session",
			Agent: sel.TargetAgent,
			Path:  sel.TargetPath,
		}}
	}
	return p
}

func projectFrom(conv *models.Conversation) string {
	if conv.Metadata == nil {
		return ""
	}
	for _, key := range []string{"cwd", "project", "directory"} {
		if s, ok := conv.Metadata[key].(string); ok {
			if v := strings.TrimSpace(s); v != "" {
				return v
			}
		}
	}
	return ""
}

func lastUsefulText(conv *models.Conversation) string {
	for i := len(conv.Messages) - 1; i >= 0; i-- {
		msg := conv.Messages[i]
		if msg.Role == models.RoleSystem {
			continue
		}
		text := strings.TrimSpace(msg.StringContent())
		if text == "" {
			continue
		}
		return clip(text, 480)
	}
	return ""
}

func clip(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}
