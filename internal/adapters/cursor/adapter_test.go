package cursor

import (
	"os"
	"path/filepath"
	"strings"
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
	if conv.Messages[0].Content != "hello cursor" {
		t.Fatalf("user content kept wrapper: %q", conv.Messages[0].Content)
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("n=%d", len(conv.Messages))
	}
	asst := conv.Messages[1]
	if len(asst.Parts) != 3 || asst.Parts[0].Kind != models.PartThinking || asst.Parts[2].Name != "Read" {
		t.Fatalf("parts: %+v", asst.Parts)
	}
}

func TestExtractMergesConsecutiveAssistantChunks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CURSOR_PROJECTS_DIR", root)
	dir := filepath.Join(root, "p", "agent-transcripts", "id")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "id.jsonl")
	body := `{"role":"user","message":{"content":[{"type":"text","text":"<user_query>q</user_query>"}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"one"}]}}
{"role":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"path":"a.go"}}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"two"}]}}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	conv, err := NewAdapter().Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(conv.Messages) != 2 {
		t.Fatalf("n=%d %#v", len(conv.Messages), conv.Messages)
	}
	asst := conv.Messages[1]
	if len(asst.Parts) != 3 || asst.Parts[0].Text != "one" || asst.Parts[2].Text != "two" {
		t.Fatalf("merged: %+v", asst.Parts)
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

func TestInjectExtractRoundTrip(t *testing.T) {
	t.Setenv("CURSOR_PROJECTS_DIR", t.TempDir())
	a := NewAdapter()
	in := &models.Conversation{
		Messages: []models.Message{
			models.NewMessage(models.RoleSystem, []models.Part{models.TextPart("be terse")}),
			models.NewMessage(models.RoleUser, []models.Part{models.TextPart("hello cursor")}),
			models.NewMessage(models.RoleAssistant, []models.Part{
				models.ThinkingPart("plan"),
				models.TextPart("hi"),
				models.ToolCallPart("tu1", "Read", `{"path":"a.go"}`),
			}),
			models.NewMessage(models.RoleUser, []models.Part{
				models.ToolResultPart("tu1", "ok", false),
			}),
		},
		Metadata: map[string]interface{}{"cwd": "/home/aiden/extra/git/cbld"},
	}
	out, err := a.Inject(in, "")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(out) {
		t.Fatalf("path %q", out)
	}
	got, err := a.Extract(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 4 {
		t.Fatalf("n=%d %#v", len(got.Messages), got.Messages)
	}
	if got.Messages[0].Role != models.RoleSystem || got.Messages[0].Content != "be terse" {
		t.Fatalf("system: %+v", got.Messages[0])
	}
	if got.Messages[1].Role != models.RoleUser || got.Messages[1].Content != "hello cursor" {
		t.Fatalf("user: %+v", got.Messages[1])
	}
	asst := got.Messages[2]
	if len(asst.Parts) != 3 || asst.Parts[0].Kind != models.PartThinking || asst.Parts[2].Name != "Read" {
		t.Fatalf("asst: %+v", asst.Parts)
	}
	if got.Title != "hello cursor" {
		t.Fatalf("title %q", got.Title)
	}
}

func TestDefaultTargetUsesProjectsDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CURSOR_PROJECTS_DIR", root)
	got, err := NewAdapter().DefaultTarget(&models.Conversation{
		Metadata: map[string]interface{}{"cwd": "/home/aiden/extra/git/cbld"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, filepath.Join(root, "home-aiden-extra-git-cbld", "agent-transcripts")) {
		t.Fatalf("target %q", got)
	}
	if filepath.Base(got) != filepath.Base(filepath.Dir(got))+".jsonl" {
		t.Fatalf("id file mismatch %q", got)
	}
}

func TestEncodeCursorProject(t *testing.T) {
	if got := encodeCursorProject("/home/aiden/extra/git/cbld"); got != "home-aiden-extra-git-cbld" {
		t.Fatalf("unix: %q", got)
	}
	if got := encodeCursorProject("C:/Users/foo"); got != "C--Users-foo" {
		t.Fatalf("windows: %q", got)
	}
	if got := encodeCursorProject("/"); got != "workspace" {
		t.Fatalf("root: %q", got)
	}
}
