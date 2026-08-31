package cmd

import (
	"strings"
	"testing"

	"vibeporter/internal/models"
)

func TestRenderHTMLEscapesConversationContent(t *testing.T) {
	conv := &models.Conversation{
		Title:       `<img src=x onerror=alert(1)>`,
		AgentSource: `<script>alert(2)</script>`,
		ID:          `"><script>alert(3)</script>`,
		Messages: []models.Message{models.NewMessage(models.RoleAssistant, []models.Part{
			models.TextPart(`<script>alert(4)</script>`),
			models.ToolCallPart("x", `"><img src=x onerror=alert(5)>`, `{"arg":"<script>alert(6)</script>"}`),
			models.ToolResultPart("x", `<img src=x onerror=alert(7)>`, false),
		})},
	}

	out := renderHTML(conv)
	if strings.Contains(out, "<script>alert") || strings.Contains(out, "<img src=x") {
		t.Fatalf("unescaped HTML content: %s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") || !strings.Contains(out, "&lt;img") {
		t.Fatalf("escaped content missing: %s", out)
	}
}
