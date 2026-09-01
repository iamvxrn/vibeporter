package compact

import (
	"testing"
	"vibeporter/internal/models"
)

func TestParseBudget(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{{"50k", 50000}, {"200k", 200000}, {"123", 123}} {
		got, err := ParseBudget(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("%q: %d %v", tc.in, got, err)
		}
	}
	if _, err := ParseBudget("many"); err == nil {
		t.Fatal("expected invalid budget")
	}
}

func TestCompactPreservesSourceAndUnicode(t *testing.T) {
	source := &models.Conversation{Messages: []models.Message{{Role: models.RoleSystem, Content: "rules"}, {Role: models.RoleUser, Content: "задача 💥 " + string(make([]rune, 200))}, {Role: models.RoleUser, Content: "unfinished request"}}}
	before := source.Messages[1].Content
	got, report, err := Compact(source, 30, Smart)
	if err != nil || report.Result > 30 || source.Messages[1].Content != before {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if got.Messages[len(got.Messages)-1].Content != "unfinished request" {
		t.Fatalf("last request lost: %+v", got.Messages)
	}
}

func TestRecentKeepsValidToolPairs(t *testing.T) {
	source := &models.Conversation{Messages: []models.Message{{Role: models.RoleAssistant, Parts: []models.Part{models.ToolCallPart("a", "read", "{}")}}, {Role: models.RoleUser, Parts: []models.Part{models.ToolResultPart("a", "output", false)}}}}
	got, _, err := Compact(source, 4, Recent)
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]bool{}
	for _, msg := range got.Messages {
		for _, part := range msg.Parts {
			if part.Kind == models.PartToolCall {
				calls[part.ID] = true
			}
		}
	}
	for _, msg := range got.Messages {
		for _, part := range msg.Parts {
			if part.Kind == models.PartToolResult && !calls[part.ToolCallID] {
				t.Fatal("orphan result retained")
			}
		}
	}
}
