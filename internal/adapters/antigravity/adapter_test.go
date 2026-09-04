package antigravity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vibeporter/internal/models"
)

func TestSafePrefix(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		want string
	}{
		{"abcdef", 3, "abc"},
		{"ab", 3, "ab"},
		{"", 3, ""},
		{"abc", 0, ""},
		{"abc", 8, "abc"},
		{"12345678", 8, "12345678"},
		{"123456789", 8, "12345678"},
	}
	for _, tt := range tests {
		got := safePrefix(tt.in, tt.n)
		if got != tt.want {
			t.Errorf("safePrefix(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
		}
	}
}

func TestExtractUserRequest(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "xml tags",
			in:   "<USER_REQUEST>\nhello world\n</USER_REQUEST>\n<ADDITIONAL_METADATA>stuff</ADDITIONAL_METADATA>",
			want: "hello world",
		},
		{
			name: "plain text",
			in:   "just a message",
			want: "just a message",
		},
		{
			name: "xml prefix lines",
			in:   "<SYSTEM>stuff</SYSTEM>\nactual message",
			want: "actual message",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUserRequest(tt.in)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveIDFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/.gemini/antigravity/brain/abc-123/handoff_context.md", "abc-123"},
		{"/home/user/.gemini/antigravity/brain/abc-123/.system_generated/logs/transcript.jsonl", "abc-123"},
		{"/tmp/random/file.md", "random"},
		{"/tmp/convdir/handoff_context.md", "convdir"},
	}
	for _, tt := range tests {
		got := deriveIDFromPath(tt.path)
		if got != tt.want {
			t.Errorf("deriveIDFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestRoundTripTranscript(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTIGRAVITY_BRAIN_DIR", dir)

	ts := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	original := &models.Conversation{
		ID:          "test-conv-id",
		AgentSource: "opencode",
		Title:       "Test Conversation",
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: "System context message", Parts: []models.Part{models.TextPart("System context message")}, Timestamp: &ts},
			{Role: models.RoleUser, Content: "Hello, fix the bug", Parts: []models.Part{models.TextPart("Hello, fix the bug")}, Timestamp: &ts},
			{Role: models.RoleAssistant, Content: "I'll look into it", Parts: []models.Part{models.TextPart("I'll look into it")}, Timestamp: &ts},
		},
		Metadata: map[string]interface{}{},
	}

	// Write transcript.jsonl
	transcriptPath := filepath.Join(dir, "test-conv-id", ".system_generated", "logs", "transcript.jsonl")
	if err := writeTranscriptJSONL(original, transcriptPath); err != nil {
		t.Fatalf("writeTranscriptJSONL: %v", err)
	}

	// Read it back
	adapter := NewAdapter()
	conv, err := adapter.Extract(transcriptPath)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Check round-trip preserved messages
	if len(conv.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(conv.Messages))
	}
	if conv.Messages[0].Role != models.RoleSystem {
		t.Errorf("message 0: expected system role, got %s", conv.Messages[0].Role)
	}
	if !strings.Contains(conv.Messages[0].Content, "System context message") {
		t.Errorf("message 0: system content lost, got %q", conv.Messages[0].Content)
	}
	if conv.Messages[1].Role != models.RoleUser {
		t.Errorf("message 1: expected user role, got %s", conv.Messages[1].Role)
	}
	if conv.Messages[2].Role != models.RoleAssistant {
		t.Errorf("message 2: expected assistant role, got %s", conv.Messages[2].Role)
	}
}

func TestRoundTripHandoffMD(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTIGRAVITY_BRAIN_DIR", dir)

	original := &models.Conversation{
		ID:          "md-test",
		AgentSource: "opencode",
		Title:       "Bug Fix Session",
		Messages: []models.Message{
			{Role: models.RoleUser, Content: "Fix the login bug", Parts: []models.Part{models.TextPart("Fix the login bug")}},
			{Role: models.RoleAssistant, Content: "I found the issue in auth.go", Parts: []models.Part{models.TextPart("I found the issue in auth.go")}},
		},
		Metadata: map[string]interface{}{},
	}

	mdPath := filepath.Join(dir, "md-test", "handoff_context.md")
	if err := writeHandoffMarkdown(original, mdPath); err != nil {
		t.Fatalf("writeHandoffMarkdown: %v", err)
	}

	// Verify the file exists and contains expected content
	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Bug Fix Session") {
		t.Error("markdown missing title")
	}
	if !strings.Contains(content, "Fix the login bug") {
		t.Error("markdown missing user message")
	}
	if !strings.Contains(content, "I found the issue in auth.go") {
		t.Error("markdown missing assistant message")
	}
	if !strings.Contains(content, "opencode") {
		t.Error("markdown missing source agent")
	}

	// Extract it back
	adapter := NewAdapter()
	conv, err := adapter.Extract(mdPath)
	if err != nil {
		t.Fatalf("Extract MD: %v", err)
	}
	if conv.Title != "Bug Fix Session" {
		t.Errorf("title: got %q, want %q", conv.Title, "Bug Fix Session")
	}
	// MD round-trip gives one system message with the full content
	if len(conv.Messages) != 1 {
		t.Fatalf("expected 1 message (system), got %d", len(conv.Messages))
	}
	if conv.Messages[0].Role != models.RoleSystem {
		t.Errorf("expected system role, got %s", conv.Messages[0].Role)
	}
}

func TestInjectCreatesMarkdown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTIGRAVITY_BRAIN_DIR", dir)

	conv := &models.Conversation{
		ID:          "inject-test",
		AgentSource: "claudecode",
		Title:       "My Session",
		Messages: []models.Message{
			{Role: models.RoleUser, Content: "Do something", Parts: []models.Part{models.TextPart("Do something")}},
			{Role: models.RoleAssistant, Content: "Done!", Parts: []models.Part{models.TextPart("Done!")}},
		},
		Metadata: map[string]interface{}{},
	}

	adapter := NewAdapter()
	written, err := adapter.Inject(conv, "")
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}

	if !strings.HasSuffix(written, "handoff_context.md") {
		t.Errorf("expected .md file, got %q", written)
	}

	data, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "My Session") {
		t.Error("missing title in output")
	}
	if !strings.Contains(content, "Do something") {
		t.Error("missing user message in output")
	}
}

func TestInjectWithToolCalls(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTIGRAVITY_BRAIN_DIR", dir)

	args := map[string]string{"path": "/tmp/test.go"}
	argsJSON, _ := json.Marshal(args)

	conv := &models.Conversation{
		ID:          "tool-test",
		AgentSource: "opencode",
		Title:       "Tool Call Session",
		Messages: []models.Message{
			{Role: models.RoleUser, Content: "Read the file", Parts: []models.Part{models.TextPart("Read the file")}},
			{
				Role:    models.RoleAssistant,
				Content: "[Tool Use: read_file]",
				Parts: []models.Part{
					models.ThinkingPart("Let me read the file"),
					models.ToolCallPart("call-1", "read_file", string(argsJSON)),
				},
			},
		},
		Metadata: map[string]interface{}{},
	}

	adapter := NewAdapter()
	written, err := adapter.Inject(conv, "")
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}

	data, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "read_file") {
		t.Error("missing tool call name")
	}
	if !strings.Contains(content, "Thinking") {
		t.Error("missing thinking section")
	}
}

func TestListConversationsIncludesHandoffMD(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTIGRAVITY_BRAIN_DIR", dir)

	// Create a conversation with a handoff markdown
	convDir := filepath.Join(dir, "handoff-conv-123")
	if err := os.MkdirAll(convDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mdPath := filepath.Join(convDir, "handoff_context.md")
	if err := os.WriteFile(mdPath, []byte("# Test Handoff\n\nSome content"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := NewAdapter()
	chats, err := adapter.ListConversations()
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}

	found := false
	for _, c := range chats {
		if c.ID == "handoff-conv-123" {
			found = true
			if c.Title != "Test Handoff" {
				t.Errorf("title: got %q, want %q", c.Title, "Test Handoff")
			}
			if !strings.HasSuffix(c.Path, "handoff_context.md") {
				t.Errorf("path: got %q, want suffix handoff_context.md", c.Path)
			}
		}
	}
	if !found {
		t.Error("handoff conversation not found in list")
	}
}
