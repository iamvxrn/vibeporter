package models

import (
	"strings"
	"time"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

type PartKind string

const (
	PartText       PartKind = "text"
	PartThinking   PartKind = "thinking"
	PartToolCall   PartKind = "tool_call"
	PartToolResult PartKind = "tool_result"
)

// Part is one block inside a Message. Thinking is kept for round-trip but
// omitted from StringContent (titles / list previews).
type Part struct {
	Kind       PartKind `json:"kind"`
	Text       string   `json:"text,omitempty"`
	ID         string   `json:"id,omitempty"`
	Name       string   `json:"name,omitempty"`
	ArgsJSON   string   `json:"args_json,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
	IsError    bool     `json:"is_error,omitempty"`
}

func TextPart(text string) Part {
	return Part{Kind: PartText, Text: text}
}

func ThinkingPart(thought string) Part {
	return Part{Kind: PartThinking, Text: thought}
}

func ToolCallPart(id, name, argsJSON string) Part {
	return Part{Kind: PartToolCall, ID: id, Name: name, ArgsJSON: argsJSON}
}

func ToolResultPart(toolCallID, content string, isError bool) Part {
	return Part{Kind: PartToolResult, ToolCallID: toolCallID, Text: content, IsError: isError}
}

type Message struct {
	Role      Role                   `json:"role"`
	Content   string                 `json:"content"`
	Parts     []Part                 `json:"parts,omitempty"`
	Timestamp *time.Time             `json:"timestamp,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// StringContent is the fallback plain-text view: text parts plus tool markers.
// Thinking is excluded so list titles stay as the user's words.
func (m Message) StringContent() string {
	if len(m.Parts) == 0 {
		return m.Content
	}
	var b strings.Builder
	write := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s)
	}
	for _, p := range m.Parts {
		switch p.Kind {
		case PartText:
			write(p.Text)
		case PartToolCall:
			name := strings.TrimSpace(p.Name)
			if name == "" {
				name = "tool"
			}
			write("[Tool Use: " + name + "]")
		case PartToolResult:
			if strings.TrimSpace(p.Text) != "" {
				write("[Tool Result]\n" + strings.TrimSpace(p.Text))
			} else {
				write("[Tool Result]")
			}
		}
	}
	return b.String()
}

// EffectiveParts returns structured parts, or a single text part from Content.
func (m Message) EffectiveParts() []Part {
	if len(m.Parts) > 0 {
		return m.Parts
	}
	if strings.TrimSpace(m.Content) == "" {
		return nil
	}
	return []Part{TextPart(m.Content)}
}

func (m *Message) SyncContent() {
	m.Content = m.StringContent()
}

func NewMessage(role Role, parts []Part) Message {
	msg := Message{Role: role, Parts: parts}
	msg.SyncContent()
	return msg
}

type Conversation struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title,omitempty"`
	AgentSource string                 `json:"agent_source"`
	Messages    []Message              `json:"messages"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
