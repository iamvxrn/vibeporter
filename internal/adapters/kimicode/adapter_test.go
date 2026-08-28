package kimicode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibeporter/internal/models"
)

func TestExtractWirePromptsAndAssistant(t *testing.T) {
	dir := t.TempDir()
	wire := filepath.Join(dir, "sessions", "wd_x", "session_abc", "agents", "main", "wire.jsonl")
	if err := os.MkdirAll(filepath.Dir(wire), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"type":"metadata","protocol_version":1}`,
		`{"type":"turn.prompt","time":1000,"origin":{"kind":"user"},"input":"hello kimi"}`,
		`{"type":"context.append_loop_event","time":1001,"event":{"type":"content.part","part":{"type":"text","text":"hi there"}}}`,
		`{"type":"context.append_loop_event","time":1002,"event":{"type":"tool.call","name":"Read"}}`,
	}, "\n")
	if err := os.WriteFile(wire, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	conv, err := extractWire(wire)
	if err != nil {
		t.Fatal(err)
	}
	if conv.ID != "session_abc" {
		t.Fatalf("id %q", conv.ID)
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("messages %d %#v", len(conv.Messages), conv.Messages)
	}
	if conv.Messages[0].Content != "hello kimi" {
		t.Fatalf("user %q", conv.Messages[0].Content)
	}
	if !strings.Contains(conv.Messages[1].Content, "hi there") {
		t.Fatalf("asst %q", conv.Messages[1].Content)
	}
	if len(conv.Messages[1].Parts) != 2 || conv.Messages[1].Parts[1].Kind != models.PartToolCall || conv.Messages[1].Parts[1].Name != "Read" {
		t.Fatalf("tool part: %+v", conv.Messages[1].Parts)
	}
}

func TestInjectThenExtract(t *testing.T) {
	t.Setenv("KIMI_CODE_HOME", t.TempDir())
	a := NewAdapter()
	conv, err := extractWire(mustWrite(t, `{"type":"turn.prompt","origin":{"kind":"user"},"input":"q"}`+"\n"+`{"type":"context.append_loop_event","event":{"type":"content.part","part":{"type":"text","text":"a"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	conv.Metadata = map[string]interface{}{"cwd": "/tmp/proj"}
	out, err := a.Inject(conv, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Extract(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("roundtrip %d", len(got.Messages))
	}
}

func TestInjectExtractSystem(t *testing.T) {
	t.Setenv("KIMI_CODE_HOME", t.TempDir())
	a := NewAdapter()
	in := &models.Conversation{
		Messages: []models.Message{
			models.NewMessage(models.RoleSystem, []models.Part{models.TextPart("be terse")}),
			models.NewMessage(models.RoleUser, []models.Part{models.TextPart("q")}),
			models.NewMessage(models.RoleAssistant, []models.Part{models.TextPart("a")}),
		},
		Metadata: map[string]interface{}{"cwd": "/tmp/proj"},
	}
	out, err := a.Inject(in, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Extract(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("n=%d %#v", len(got.Messages), got.Messages)
	}
	if got.Messages[0].Role != models.RoleSystem || got.Messages[0].Content != "be terse" {
		t.Fatalf("system: %+v", got.Messages[0])
	}
}

func mustWrite(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "sessions", "wd", "session_x", "agents", "main", "wire.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
