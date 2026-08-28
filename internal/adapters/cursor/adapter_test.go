package cursor

import (
	"os"
	"path/filepath"
	"testing"

	"vibeporter/internal/models"
)

func TestExtractListAgentTranscript(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CURSOR_PROJECTS_DIR", root)
	dir := filepath.Join(root, "my-proj", "agent-transcripts", "aaa-111")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "aaa-111.jsonl")
	body := `{"role":"user","message":{"content":[{"type":"text","text":"<user_query>hello cursor</user_query>"}]}}
{"role":"assistant","message":{"content":[{"type":"thinking","thinking":"plan"},{"type":"text","text":"hi"},{"type":"tool_use","name":"Read","input":{"path":"a.go"}}]}}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	a := NewAdapter()
	chats, err := a.ListConversations()
	if err != nil || len(chats) != 1 {
		t.Fatalf("list: %+v err=%v", chats, err)
	}
	if chats[0].ID != "aaa-111" || chats[0].Title != "hello cursor" {
		t.Fatalf("chat: %+v", chats[0])
	}
	conv, err := a.Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if conv.Title != "hello cursor" || conv.AgentSource != "cursor" {
		t.Fatalf("meta: %+v", conv)
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("n=%d", len(conv.Messages))
	}
	asst := conv.Messages[1]
	if len(asst.Parts) != 3 || asst.Parts[0].Kind != models.PartThinking || asst.Parts[2].Name != "Read" {
		t.Fatalf("parts: %+v", asst.Parts)
	}
}

func TestListSkipsSubagents(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CURSOR_PROJECTS_DIR", root)
	sub := filepath.Join(root, "p", "agent-transcripts", "id", "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "child.jsonl"), []byte(`{"role":"user","message":{"content":"x"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	chats, err := NewAdapter().ListConversations()
	if err != nil || len(chats) != 0 {
		t.Fatalf("subagents should be skipped: %+v %v", chats, err)
	}
}
