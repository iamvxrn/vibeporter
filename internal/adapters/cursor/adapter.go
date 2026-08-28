// Package cursor extracts and injects Cursor agent transcripts.
//
// Cursor stores agent chats as JSONL under
// ~/.cursor/projects/<project>/agent-transcripts/<id>/<id>.jsonl
// Override the projects root with CURSOR_PROJECTS_DIR. Inject writes a new
// session and never updates an existing one. Subagent transcripts are skipped.
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
		case "system":
			role = models.RoleSystem
		default:
			continue
		}
		parts := cursorParts(rec)
		if role == models.RoleUser {
			parts = stripUserTextParts(parts)
		}
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

func stripUserTextParts(parts []models.Part) []models.Part {
	var out []models.Part
	for _, p := range parts {
		if p.Kind == models.PartText {
			p.Text = stripUserQueryWrapper(p.Text)
			if strings.TrimSpace(p.Text) == "" {
				continue
			}
		}
		out = append(out, p)
	}
	return out
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

func wrapUserQuery(text string) string {
	if strings.Contains(text, "<user_query>") {
		return text
	}
	return "<user_query>\n" + strings.TrimSpace(text) + "\n</user_query>"
}

func encodeCursorProject(cwd string) string {
	s := filepath.ToSlash(filepath.Clean(cwd))
	s = strings.Trim(s, "/")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	if s == "" || s == "." {
		return "workspace"
	}
	return s
}

func (a *Adapter) DefaultTarget(conv *models.Conversation) (string, error) {
	cwd := adapters.CwdFromMeta(conv)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	id := adapters.NewUUID()
	dir := filepath.Join(projectsRoot(), encodeCursorProject(cwd), "agent-transcripts", id)
	return filepath.Join(dir, id+".jsonl"), nil
}

func (a *Adapter) Inject(conv *models.Conversation, targetPath string) (string, error) {
	var err error
	if strings.TrimSpace(targetPath) == "" {
		targetPath, err = a.DefaultTarget(conv)
		if err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	w := bufio.NewWriter(f)
	for _, msg := range conv.Messages {
		role := "assistant"
		switch msg.Role {
		case models.RoleUser:
			role = "user"
		case models.RoleSystem:
			role = "system"
		}
		if err := writeJSONLine(w, map[string]interface{}{
			"role": role,
			"message": map[string]interface{}{
				"content": cursorContentBlocks(msg),
			},
		}); err != nil {
			return "", err
		}
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	return targetPath, nil
}

func cursorContentBlocks(msg models.Message) []map[string]interface{} {
	wrap := msg.Role == models.RoleUser
	var out []map[string]interface{}
	for _, p := range msg.EffectiveParts() {
		switch p.Kind {
		case models.PartText:
			text := p.Text
			if wrap {
				text = wrapUserQuery(text)
			}
			out = append(out, map[string]interface{}{"type": "text", "text": text})
		case models.PartThinking:
			out = append(out, map[string]interface{}{"type": "thinking", "thinking": p.Text})
		case models.PartToolCall:
			var input interface{} = map[string]interface{}{}
			if strings.TrimSpace(p.ArgsJSON) != "" {
				_ = json.Unmarshal([]byte(p.ArgsJSON), &input)
			}
			id := p.ID
			if id == "" {
				id = adapters.NewUUID()
			}
			out = append(out, map[string]interface{}{
				"type": "tool_use", "id": id, "name": p.Name, "input": input,
			})
		case models.PartToolResult:
			out = append(out, map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": p.ToolCallID,
				"content":     p.Text,
				"is_error":    p.IsError,
			})
		}
	}
	if len(out) == 0 {
		out = []map[string]interface{}{{"type": "text", "text": ""}}
	}
	return out
}

func writeJSONLine(w *bufio.Writer, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	return w.WriteByte('\n')
}
