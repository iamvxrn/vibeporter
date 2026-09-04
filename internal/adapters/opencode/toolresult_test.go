package opencode

import (
	"path/filepath"
	"strings"
	"testing"

	"vibeporter/internal/models"
)

// Inject folds each tool result into its tool call's state.output. Extract used
// to read only state.input, so every command output, file read and search
// result in an OpenCode session was silently dropped: `handoff --from opencode`
// carried the tool calls but none of their answers, `search` could not find
// text that appeared in tool output, and `stats` reported zero tool results.
func TestExtractRecoversToolOutput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))

	conv := &models.Conversation{
		ID: "src", Title: "T", AgentSource: "synthetic",
		Messages: []models.Message{
			models.NewMessage(models.RoleUser, []models.Part{models.TextPart("what is in main.go?")}),
			models.NewMessage(models.RoleAssistant, []models.Part{
				models.ToolCallPart("call_1", "Read", `{"path":"main.go"}`),
			}),
			models.NewMessage(models.RoleUser, []models.Part{
				models.ToolResultPart("call_1", "package main // TOOL-OUTPUT-MARK", false),
			}),
		},
	}

	a := NewAdapter()
	written, err := a.Inject(conv, "")
	if err != nil {
		t.Fatal(err)
	}
	round, err := a.Extract(written)
	if err != nil {
		t.Fatal(err)
	}

	var results []models.Part
	for _, m := range round.Messages {
		for _, p := range m.EffectiveParts() {
			if p.Kind == models.PartToolResult {
				results = append(results, p)
			}
		}
	}
	if len(results) != 1 {
		t.Fatalf("got %d tool_result parts, want 1 (round-tripped messages: %+v)", len(results), round.Messages)
	}
	if !strings.Contains(results[0].Text, "TOOL-OUTPUT-MARK") {
		t.Fatalf("tool output not preserved: %q", results[0].Text)
	}
	if results[0].ToolCallID != "call_1" {
		t.Errorf("tool result lost its call id: %q", results[0].ToolCallID)
	}
	if results[0].IsError {
		t.Errorf("successful tool result marked as error")
	}
}

// A failed tool call's output must survive too, and stay flagged as an error.
func TestExtractRecoversToolError(t *testing.T) {
	raw := `{"type":"tool","tool":"Bash","callID":"c9","state":{"status":"error","input":{"cmd":"false"},"output":"exit status 1: BOOM-MARK"}}`
	parts := parseOpenCodeParts(raw)
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want tool_call + tool_result: %+v", len(parts), parts)
	}
	if parts[1].Kind != models.PartToolResult || !strings.Contains(parts[1].Text, "BOOM-MARK") {
		t.Fatalf("error output lost: %+v", parts[1])
	}
	if !parts[1].IsError {
		t.Errorf("failed tool result not flagged as an error")
	}
}

// A tool call with no output must not gain an empty tool_result part.
func TestExtractSkipsEmptyToolOutput(t *testing.T) {
	raw := `{"type":"tool","tool":"Read","callID":"c1","state":{"status":"completed","input":{"p":"a"},"output":""}}`
	parts := parseOpenCodeParts(raw)
	if len(parts) != 1 {
		t.Fatalf("got %d parts, want only the tool_call: %+v", len(parts), parts)
	}
}
