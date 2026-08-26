package models

import "time"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

type Message struct {
	Role      Role                   `json:"role"`
	Content   string                 `json:"content"`
	Timestamp *time.Time             `json:"timestamp,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type Conversation struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title,omitempty"`
	AgentSource string                 `json:"agent_source"`
	Messages    []Message              `json:"messages"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
