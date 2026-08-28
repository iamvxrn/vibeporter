package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRoleConstants(t *testing.T) {
	if RoleUser != "user" || RoleAssistant != "assistant" || RoleSystem != "system" {
		t.Fatalf("roles: %q %q %q", RoleUser, RoleAssistant, RoleSystem)
	}
}

func TestConversationJSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	in := Conversation{
		ID:          "c1",
		Title:       "hello",
		AgentSource: "gemini",
		Messages: []Message{
			{Role: RoleUser, Content: "hi", Timestamp: &ts, Metadata: map[string]interface{}{"k": "v"}},
		},
		Metadata: map[string]interface{}{"cwd": "/tmp"},
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Conversation
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != in.ID || out.Title != in.Title || out.AgentSource != in.AgentSource {
		t.Fatalf("meta: %+v", out)
	}
	if len(out.Messages) != 1 || out.Messages[0].Role != RoleUser || out.Messages[0].Content != "hi" {
		t.Fatalf("messages: %+v", out.Messages)
	}
	if out.Metadata["cwd"] != "/tmp" {
		t.Fatalf("metadata: %+v", out.Metadata)
	}
}

func TestStringContentAndEffectiveParts(t *testing.T) {
	plain := Message{Role: RoleUser, Content: "hi"}
	if plain.StringContent() != "hi" {
		t.Fatalf("legacy: %q", plain.StringContent())
	}
	if parts := plain.EffectiveParts(); len(parts) != 1 || parts[0].Text != "hi" {
		t.Fatalf("effective from content: %+v", parts)
	}

	msg := NewMessage(RoleAssistant, []Part{
		ThinkingPart("secret"),
		TextPart("ok"),
		ToolCallPart("c1", "bash", `{"cmd":"ls"}`),
		ToolResultPart("c1", "a.txt", false),
	})
	if msg.Content != "ok\n[Tool Use: bash]\n[Tool Result]\na.txt" {
		t.Fatalf("content: %q", msg.Content)
	}
	if len(msg.Parts) != 4 || msg.Parts[0].Kind != PartThinking {
		t.Fatalf("parts: %+v", msg.Parts)
	}
	if TextPart("x").Kind != PartText || ThinkingPart("t").Kind != PartThinking {
		t.Fatal("constructors")
	}
}
