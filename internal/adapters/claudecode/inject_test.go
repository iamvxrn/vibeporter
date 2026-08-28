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

func TestExtractInjectToolAndThinking(t *testing.T) {
	a := NewAdapter()
	in := &models.Conversation{
		Title: "tools",
		Messages: []models.Message{
			models.NewMessage(models.RoleUser, []models.Part{models.TextPart("run ls")}),
			models.NewMessage(models.RoleAssistant, []models.Part{
				models.ThinkingPart("plan"),
				models.TextPart("ok"),
				models.ToolCallPart("tu1", "Bash", `{"command":"ls"}`),
			}),
			models.NewMessage(models.RoleUser, []models.Part{
				models.ToolResultPart("tu1", "a.txt", false),
			}),
		},
	}
	out, err := a.Inject(in, filepath.Join(t.TempDir(), "sess.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Extract(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("n=%d", len(got.Messages))
	}
	asst := got.Messages[1]
	if len(asst.Parts) != 3 {
		t.Fatalf("asst parts: %+v", asst.Parts)
	}
	if asst.Parts[0].Kind != models.PartThinking || asst.Parts[0].Text != "plan" {
		t.Fatalf("thinking: %+v", asst.Parts[0])
	}
	if asst.Parts[2].Kind != models.PartToolCall || asst.Parts[2].Name != "Bash" {
		t.Fatalf("tool: %+v", asst.Parts[2])
	}
	if got.Messages[2].Parts[0].Kind != models.PartToolResult || got.Messages[2].Parts[0].Text != "a.txt" {
		t.Fatalf("result: %+v", got.Messages[2].Parts)
	}
}

func TestEncodeClaudeProjectReplacesDriveColon(t *testing.T) {
	got := encodeClaudeProject("C:/Users/foo")
	if got != "-C--Users-foo" {
		t.Fatalf("windows path: %q", got)
	}
	unix := encodeClaudeProject("/home/foo/proj")
	if unix != "-home-foo-proj" {
		t.Fatalf("unix path: %q", unix)
	}
}
