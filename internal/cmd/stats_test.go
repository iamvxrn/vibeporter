package cmd

import (
	"strings"
	"testing"

	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

func TestCollectStats(t *testing.T) {
	fake := fakeSearchExtractor{
		chats: []adapters.ChatInfo{
			{ID: "a", Title: "one", Path: "p1"},
			{ID: "b", Title: "two", Path: "p2"},
		},
		convs: map[string]*models.Conversation{
			"p1": {
				ID: "a",
				Messages: []models.Message{
					models.NewMessage(models.RoleUser, []models.Part{models.TextPart("hello world")}),
					models.NewMessage(models.RoleAssistant, []models.Part{models.ThinkingPart("think"), models.ToolCallPart("1", "tool", `{"a":1}`)}),
				},
			},
			"p2": {
				ID: "b",
				Messages: []models.Message{
					models.NewMessage(models.RoleUser, []models.Part{models.TextPart("hi")}),
				},
			},
		},
	}
	orig, had := extractors["statstest"]
	extractors["statstest"] = fake
	defer func() {
		if had {
			extractors["statstest"] = orig
		} else {
			delete(extractors, "statstest")
		}
	}()

	rows, err := collectStats([]string{"statstest"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows %d", len(rows))
	}
	r := rows[0]
	if r.Chats != 2 || r.Messages != 3 {
		t.Fatalf("chats %d msgs %d", r.Chats, r.Messages)
	}
	if r.TextParts != 2 {
		t.Fatalf("text %d", r.TextParts)
	}
	if r.Thinking != 1 || r.ToolCalls != 1 {
		t.Fatalf("think %d tools %d", r.Thinking, r.ToolCalls)
	}
	if r.Chars == 0 || r.TokensEst == 0 {
		t.Fatalf("chars %d tokens %d", r.Chars, r.TokensEst)
	}
	// tokens ~ chars/4
	if r.TokensEst != (r.Chars+3)/4 {
		t.Fatalf("tokens %d chars %d", r.TokensEst, r.Chars)
	}
}

func TestPrintStatsHuman(t *testing.T) {
	rows := []agentStats{
		{Agent: "a", Chats: 2, Messages: 3, TextParts: 2, Thinking: 1, ToolCalls: 1, Chars: 100, TokensEst: 25},
		{Agent: "b", Chats: 0},
	}
	out := captureStdout(t, func() { printStatsHuman(rows) })
	if !strings.Contains(out, "AGENT") || !strings.Contains(out, "TOTAL") {
		t.Fatalf("out %q", out)
	}
	if !strings.Contains(out, "Chats per agent") {
		t.Fatalf("bars %q", out)
	}
	empty := captureStdout(t, func() { printStatsHuman(nil) })
	if !strings.Contains(empty, "No chats") {
		t.Fatalf("empty %q", empty)
	}
}

func TestStatsUnknownAgent(t *testing.T) {
	origAgent := statsAgent
	statsAgent = "nope"
	t.Cleanup(func() { statsAgent = origAgent })
	if err := statsCmd.RunE(statsCmd, nil); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err %v", err)
	}
}
