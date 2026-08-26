package claudecode

import (
	"os"
	"path/filepath"
	"testing"

	"vibeporter/internal/models"
)

func TestInjectRoundTrip(t *testing.T) {
	a := NewAdapter()
	target := filepath.Join(t.TempDir(), "sess.jsonl")
	conv := &models.Conversation{
		Title: "hello title",
		Messages: []models.Message{
			{Role: models.RoleUser, Content: "hello"},
			{Role: models.RoleAssistant, Content: "hi"},
		},
		Metadata: map[string]interface{}{"cwd": "/tmp/proj"},
	}
	out, err := a.Inject(conv, target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Extract(out)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "hello title" {
		t.Fatalf("title %q", got.Title)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("n=%d", len(got.Messages))
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}
