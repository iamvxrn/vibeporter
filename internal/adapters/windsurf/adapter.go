package windsurf

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

func dataRoots() []string {
	var roots []string
	if env := os.Getenv("WINDSURF_DATA_DIR"); env != "" {
		roots = append(roots, env)
	}
	home, _ := os.UserHomeDir()
	roots = append(roots,
		filepath.Join(home, ".windsurf"),
		filepath.Join(home, ".codeium", "windsurf"),
		filepath.Join(home, ".config", "windsurf"),
		filepath.Join(home, "Library", "Application Support", "Windsurf"),
	)
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		roots = append(roots, filepath.Join(xdg, "windsurf"))
	}
	return roots
}

func (a *Adapter) ListConversations() ([]adapters.ChatInfo, error) {
	var chats []adapters.ChatInfo
	for _, root := range dataRoots() {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".jsonl") && !strings.HasSuffix(path, ".json") {
				return nil
			}
			// Heuristic: windsurf transcripts contain "windsurf" or are under sessions/chats
			// For now list all jsonl under these roots
			chats = append(chats, summarizeTranscript(path, info.ModTime()))
			return nil
		})
	}
	return chats, nil
}

func summarizeTranscript(path string, mod time.Time) adapters.ChatInfo {
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	id = strings.TrimSuffix(id, ".json")
	if id == "" {
		id = filepath.Base(filepath.Dir(path))
	}
	info := adapters.ChatInfo{
		ID:        id,
		Path:      path,
		Agent:     "windsurf",
		UpdatedAt: mod,
		Title:     "Untitled",
		Project:   adapters.ShortPath(filepath.Dir(path)),
	}
	// Try to get title from first user message
	_ = adapters.ForEachJSONLLimited(path, 256*1024, 128*1024, func(rec map[string]interface{}) {
		if info.Title != "Untitled" {
			return
		}
		role, _ := rec["role"].(string)
		if role != "user" {
			// also check message.role
			if m, ok := rec["message"].(map[string]interface{}); ok {
				role, _ = m["role"].(string)
			}
		}
		if role != "user" {
			return
		}
		txt := firstText(rec)
		if txt != "" {
			info.Title = adapters.Clip(txt, 80)
		}
	})
	return info
}

func firstText(rec map[string]interface{}) string {
	if s, ok := rec["content"].(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	if m, ok := rec["message"].(map[string]interface{}); ok {
		if s, ok := m["content"].(string); ok {
			return s
		}
		if arr, ok := m["content"].([]interface{}); ok {
			for _, item := range arr {
				if mm, ok := item.(map[string]interface{}); ok {
					if t, ok := mm["text"].(string); ok && strings.TrimSpace(t) != "" {
						return t
					}
				}
			}
		}
	}
	return ""
}

func (a *Adapter) Extract(sourcePath string) (*models.Conversation, error) {
	f, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	id := strings.TrimSuffix(filepath.Base(sourcePath), ".jsonl")
	id = strings.TrimSuffix(id, ".json")
	conv := &models.Conversation{
		ID:          id,
		AgentSource: "windsurf",
		Messages:    []models.Message{},
		Metadata:    map[string]interface{}{},
	}
	sc := bufio.NewScanner(f)
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
		roleStr := ""
		if r, ok := rec["role"].(string); ok {
			roleStr = r
		} else if m, ok := rec["message"].(map[string]interface{}); ok {
			roleStr, _ = m["role"].(string)
		}
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
		parts := windsurfParts(rec)
		if len(parts) == 0 {
			continue
		}
		// Merge consecutive same-role as cursor does
		if n := len(conv.Messages); n > 0 && conv.Messages[n-1].Role == role {
			prev := &conv.Messages[n-1]
			prev.Parts = append(prev.Parts, parts...)
			prev.SyncContent()
			continue
		}
		msg := models.NewMessage(role, parts)
		if ts, ok := rec["timestamp"].(string); ok {
			msg.Timestamp = adapters.ParseTime(ts)
		} else if ts, ok := rec["created_at"].(string); ok {
			msg.Timestamp = adapters.ParseTime(ts)
		}
		conv.Messages = append(conv.Messages, msg)
	}
	for _, m := range conv.Messages {
		if m.Role == models.RoleUser && m.Content != "" {
			conv.Title = adapters.Clip(m.Content, 80)
			break
		}
	}
	return conv, sc.Err()
}

func windsurfParts(rec map[string]interface{}) []models.Part {
	// Try message.content array
	if msg, ok := rec["message"].(map[string]interface{}); ok {
		if content, ok := msg["content"]; ok {
			switch v := content.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return []models.Part{models.TextPart(v)}
				}
			case []interface{}:
				var parts []models.Part
				for _, raw := range v {
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
				if len(parts) > 0 {
					return parts
				}
			}
		}
	}
	// Fallback to top-level content
	if s, ok := rec["content"].(string); ok && strings.TrimSpace(s) != "" {
		return []models.Part{models.TextPart(s)}
	}
	if arr, ok := rec["content"].([]interface{}); ok {
		var parts []models.Part
		for _, raw := range arr {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if t, ok := m["text"].(string); ok && strings.TrimSpace(t) != "" {
				parts = append(parts, models.TextPart(t))
			}
		}
		if len(parts) > 0 {
			return parts
		}
	}
	return nil
}

func (a *Adapter) DefaultTarget(conv *models.Conversation) (string, error) {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".windsurf", "sessions")
	if env := os.Getenv("WINDSURF_DATA_DIR"); env != "" {
		base = filepath.Join(env, "sessions")
	}
	id := adapters.NewUUID()
	return filepath.Join(base, id, id+".jsonl"), nil
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
		rec := map[string]interface{}{
			"role": role,
			"message": map[string]interface{}{
				"role":    role,
				"content": windsurfContentBlocks(msg),
			},
			"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		}
		b, _ := json.Marshal(rec)
		if _, err := w.Write(b); err != nil {
			return "", err
		}
		if err := w.WriteByte('\n'); err != nil {
			return "", err
		}
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	return targetPath, nil
}

func windsurfContentBlocks(msg models.Message) []map[string]interface{} {
	var out []map[string]interface{}
	for _, p := range msg.EffectiveParts() {
		switch p.Kind {
		case models.PartText:
			out = append(out, map[string]interface{}{"type": "text", "text": p.Text})
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
			out = append(out, map[string]interface{}{"type": "tool_use", "id": id, "name": p.Name, "input": input})
		case models.PartToolResult:
			out = append(out, map[string]interface{}{"type": "tool_result", "tool_use_id": p.ToolCallID, "content": p.Text, "is_error": p.IsError})
		}
	}
	if len(out) == 0 {
		out = []map[string]interface{}{{"type": "text", "text": ""}}
	}
	return out
}
