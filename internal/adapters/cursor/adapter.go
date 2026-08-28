// Package cursor extracts Cursor agent transcripts (read-only).
//
// Cursor stores agent chats as JSONL under
// ~/.cursor/projects/<project>/agent-transcripts/<id>/<id>.jsonl
// Override the projects root with CURSOR_PROJECTS_DIR. Inject is not
// implemented: Cursor's session schema is not a stable import target.
package cursor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func projectsRoot() string {
	if env := os.Getenv("CURSOR_PROJECTS_DIR"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cursor", "projects")
}

func (a *Adapter) ListConversations() ([]adapters.ChatInfo, error) {
	root := projectsRoot()
	var chats []adapters.ChatInfo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		slash := filepath.ToSlash(path)
		if !strings.Contains(slash, "/agent-transcripts/") || strings.Contains(slash, "/subagents/") {
			return nil
		}
		chats = append(chats, summarizeTranscript(path, info.ModTime()))
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return chats, nil
}

func summarizeTranscript(path string, mod time.Time) adapters.ChatInfo {
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	info := adapters.ChatInfo{
		ID:        id,
		Path:      path,
		Agent:     "cursor",
		UpdatedAt: mod,
		Title:     "Untitled",
	}
	// .../projects/<proj>/agent-transcripts/<id>/<id>.jsonl
	proj := path
	for i := 0; i < 3; i++ {
		proj = filepath.Dir(proj)
	}
	info.Project = adapters.ShortPath(proj)

	var firstUser string
	_ = adapters.ForEachJSONLLimited(path, 256*1024, 128*1024, func(rec map[string]interface{}) {
		role, _ := rec["role"].(string)
		if role != "user" || firstUser != "" {
			return
		}
		firstUser = firstTextFromCursorMessage(rec)
	})
	if firstUser != "" {
		info.Title = adapters.Clip(stripUserQueryWrapper(firstUser), 80)
	}
	return info
}

func (a *Adapter) Extract(sourcePath string) (*models.Conversation, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	id := strings.TrimSuffix(filepath.Base(sourcePath), ".jsonl")
	conv := &models.Conversation{
		ID:          id,
		AgentSource: "cursor",
		Messages:    []models.Message{},
		Metadata:    map[string]interface{}{},
	}

	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec map[string]interface{}
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		roleStr, _ := rec["role"].(string)
		var role models.Role
		switch roleStr {
		case "user":
			role = models.RoleUser
		case "assistant":
			role = models.RoleAssistant
		default:
			continue
		}
		parts := cursorParts(rec)
		if len(parts) == 0 {
			continue
		}
		conv.Messages = append(conv.Messages, models.NewMessage(role, parts))
	}
	for _, m := range conv.Messages {
		if m.Role == models.RoleUser && m.Content != "" {
			conv.Title = adapters.Clip(stripUserQueryWrapper(m.Content), 80)
			break
		}
	}
	return conv, sc.Err()
}

func cursorParts(rec map[string]interface{}) []models.Part {
	msg, ok := rec["message"].(map[string]interface{})
	if !ok {
		if s, _ := rec["content"].(string); strings.TrimSpace(s) != "" {
			return []models.Part{models.TextPart(s)}
		}
		return nil
	}
	switch content := msg["content"].(type) {
	case string:
		if strings.TrimSpace(content) == "" {
			return nil
		}
		return []models.Part{models.TextPart(content)}
	case []interface{}:
		var parts []models.Part
		for _, raw := range content {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			switch m["type"] {
			case "text":
				if t, _ := m["text"].(string); strings.TrimSpace(t) != "" {
					parts = append(parts, models.TextPart(t))
				}
			case "thinking":
				t, _ := m["thinking"].(string)
				if t == "" {
					t, _ = m["text"].(string)
				}
				if strings.TrimSpace(t) != "" {
					parts = append(parts, models.ThinkingPart(t))
				}
			case "tool_use":
				name, _ := m["name"].(string)
				id, _ := m["id"].(string)
				args := ""
				if in := m["input"]; in != nil {
					b, _ := json.Marshal(in)
					args = string(b)
				}
				parts = append(parts, models.ToolCallPart(id, name, args))
			case "tool_result":
				id, _ := m["tool_use_id"].(string)
				text := ""
				if s, ok := m["content"].(string); ok {
					text = s
				} else if s, ok := m["text"].(string); ok {
					text = s
				}
				isErr, _ := m["is_error"].(bool)
				parts = append(parts, models.ToolResultPart(id, text, isErr))
			}
		}
		return parts
	default:
		return nil
	}
}

func firstTextFromCursorMessage(rec map[string]interface{}) string {
	for _, p := range cursorParts(rec) {
		if p.Kind == models.PartText && strings.TrimSpace(p.Text) != "" {
			return p.Text
		}
	}
	return ""
}

func stripUserQueryWrapper(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "<user_query>"); i >= 0 {
		s = s[i+len("<user_query>"):]
		if j := strings.Index(s, "</user_query>"); j >= 0 {
			s = s[:j]
		}
		return strings.TrimSpace(s)
	}
	return s
}
