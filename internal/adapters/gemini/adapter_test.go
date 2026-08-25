package gemini

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibeporter/internal/models"
)

func TestMessageRole(t *testing.T) {
	if r, keep := messageRole("user"); r != models.RoleUser || !keep {
		t.Errorf("expected user -> RoleUser (kept), got %v keep=%v", r, keep)
	}
	if r, keep := messageRole("gemini"); r != models.RoleAssistant || !keep {
		t.Errorf("expected gemini -> RoleAssistant (kept), got %v keep=%v", r, keep)
	}
	if _, keep := messageRole("info"); keep {
		t.Errorf("expected info records to be dropped")
	}
}

func TestExtractParsesJSONLSession(t *testing.T) {
	adapter := NewAdapter()
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "session-2026-01-01T00-00-abcd1234.jsonl")

	// First line: metadata. Then a user message (parts array), a gemini message
	// (string content), a $set metadata update, and an info notice to be dropped.
	mockData := strings.Join([]string{
		`{"sessionId":"sess-1","projectHash":"hash","startTime":"2026-01-01T00:00:00Z"}`,
		`{"id":"m1","timestamp":"2026-01-01T00:00:01Z","type":"user","content":[{"text":"hello"}]}`,
		`{"id":"m2","timestamp":"2026-01-01T00:00:02Z","type":"gemini","content":"hi there"}`,
		`{"$set":{"lastUpdated":"2026-01-01T00:00:03Z"}}`,
		`{"id":"m3","timestamp":"2026-01-01T00:00:04Z","type":"info","content":[{"text":"session resumed"}]}`,
	}, "\n")

	if err := os.WriteFile(sourcePath, []byte(mockData), 0o644); err != nil {
		t.Fatal(err)
	}

	conv, err := adapter.Extract(sourcePath)
	if err != nil {
		t.Fatal(err)
	}

	if conv.ID != "sess-1" {
		t.Errorf("expected conversation id from metadata, got %q", conv.ID)
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("expected 2 messages (info dropped), got %d", len(conv.Messages))
	}
	if conv.Messages[0].Role != models.RoleUser || conv.Messages[0].Content != "hello" {
		t.Errorf("first message incorrect: %+v", conv.Messages[0])
	}
	if conv.Messages[1].Role != models.RoleAssistant || conv.Messages[1].Content != "hi there" {
		t.Errorf("second message incorrect: %+v", conv.Messages[1])
	}
}

func TestExtractAppliesRewind(t *testing.T) {
	adapter := NewAdapter()
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "session.jsonl")

	mockData := strings.Join([]string{
		`{"sessionId":"s","projectHash":"h","startTime":"2026-01-01T00:00:00Z"}`,
		`{"id":"m1","type":"user","content":[{"text":"first"}]}`,
		`{"id":"m2","type":"gemini","content":"answer"}`,
		`{"$rewindTo":"m2"}`,
		`{"id":"m3","type":"user","content":[{"text":"second"}]}`,
	}, "\n")

	if err := os.WriteFile(sourcePath, []byte(mockData), 0o644); err != nil {
		t.Fatal(err)
	}

	conv, err := adapter.Extract(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("expected 2 messages after rewind, got %d", len(conv.Messages))
	}
	if conv.Messages[0].Content != "first" || conv.Messages[1].Content != "second" {
		t.Errorf("rewind produced wrong messages: %+v", conv.Messages)
	}
}

func TestInjectRoundTrips(t *testing.T) {
	adapter := NewAdapter()
	conv := &models.Conversation{
		ID:          "x",
		AgentSource: "gemini",
		Messages: []models.Message{
			{Role: models.RoleUser, Content: "q"},
			{Role: models.RoleAssistant, Content: "a"},
		},
	}

	targetPath := filepath.Join(t.TempDir(), "out.jsonl")
	if _, err := adapter.Inject(conv, targetPath); err != nil {
		t.Fatal(err)
	}

	got, err := adapter.Extract(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("expected 2 messages after round-trip, got %d", len(got.Messages))
	}
	if got.Messages[0].Role != models.RoleUser || got.Messages[0].Content != "q" {
		t.Errorf("round-trip user message incorrect: %+v", got.Messages[0])
	}
	if got.Messages[1].Role != models.RoleAssistant || got.Messages[1].Content != "a" {
		t.Errorf("round-trip assistant message incorrect: %+v", got.Messages[1])
	}
}
