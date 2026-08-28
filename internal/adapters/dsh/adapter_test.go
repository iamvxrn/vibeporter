package dsh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"vibeporter/internal/models"
)

func TestExtractSessionLog(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ses_1", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"type":"session","version":1,"id":"ses_1","cwd":"/tmp/proj","createdAt":1,"delegationDepth":0}`,
		`{"type":"user/message","seq":0,"time":2,"data":{"content":"hello dsh","source":{"kind":"user"}},"surfaceOp":"add"}`,
		`{"type":"assistant/message","seq":1,"time":3,"data":{"turn":0,"step":0,"message":{"content":"hi"}},"surfaceOp":"add"}`,
	}, "\n")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	conv, err := extractLog(p)
	if err != nil {
		t.Fatal(err)
	}
	if conv.ID != "ses_1" {
		t.Fatalf("id %q", conv.ID)
	}
	if conv.Metadata["cwd"] != "/tmp/proj" {
		t.Fatalf("cwd %#v", conv.Metadata)
	}
	if conv.Messages[0].Role != models.RoleUser || conv.Messages[0].Content != "hello dsh" {
		t.Fatalf("%#v", conv.Messages)
	}
}

func TestExtractZstdSessionLog(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ses_z", "session.jsonl.zstd")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"type":"session","version":1,"id":"ses_z","cwd":"/tmp/proj","createdAt":1,"delegationDepth":0}`,
		`{"type":"user/message","seq":0,"time":2,"data":{"content":"hello zstd","source":{"kind":"user"}},"surfaceOp":"add"}`,
	}, "\n")
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	compressed := enc.EncodeAll([]byte(body), nil)
	_ = enc.Close()
	if err := os.WriteFile(p, compressed, 0o644); err != nil {
		t.Fatal(err)
	}
	conv, err := extractLog(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(conv.Messages) != 1 || conv.Messages[0].Content != "hello zstd" {
		t.Fatalf("%#v", conv.Messages)
	}
}

func TestInjectRoundTrip(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	a := NewAdapter()
	conv := &models.Conversation{
		Title:       "t",
		AgentSource: "dsh",
		Messages: []models.Message{
			{Role: models.RoleUser, Content: "q"},
			{Role: models.RoleAssistant, Content: "a"},
		},
		Metadata: map[string]interface{}{"cwd": "/tmp/p"},
	}
	out, err := a.Inject(conv, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Extract(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("got %d", len(got.Messages))
	}
}

func TestInjectExtractSystem(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	a := NewAdapter()
	conv := &models.Conversation{
		Messages: []models.Message{
			models.NewMessage(models.RoleSystem, []models.Part{models.TextPart("be terse")}),
			models.NewMessage(models.RoleUser, []models.Part{models.TextPart("q")}),
			models.NewMessage(models.RoleAssistant, []models.Part{models.TextPart("a")}),
		},
		Metadata: map[string]interface{}{"cwd": "/tmp/p"},
	}
	out, err := a.Inject(conv, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Extract(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("got %d", len(got.Messages))
	}
	if got.Messages[0].Role != models.RoleSystem || got.Messages[0].Content != "be terse" {
		t.Fatalf("system: %+v", got.Messages[0])
	}
}
