package claudecode

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

func NewAdapter() *Adapter {
	return &Adapter{}
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
		AgentSource: "claudecode",
		Messages:    []models.Message{},
		Metadata:    map[string]interface{}{},
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal(line, &data); err != nil {
			continue
		}
		if meta, _ := data["isMeta"].(bool); meta {
			continue
		}
		if cwd, _ := data["cwd"].(string); cwd != "" {
			if _, ok := conv.Metadata["cwd"]; !ok {
				conv.Metadata["cwd"] = cwd
			}
		}
		if title, _ := data["aiTitle"].(string); strings.TrimSpace(title) != "" {
			conv.Title = strings.TrimSpace(title)
		}

		msgType, _ := data["type"].(string)
		if msgType != "user" && msgType != "assistant" {
			continue
		}
		role := models.RoleAssistant
		if msgType == "user" {
			role = models.RoleUser
		}
		parts := claudeParts(data)
		if len(parts) == 0 {
			continue
		}
		msg := models.NewMessage(role, parts)
		if isSlashCommandDump(msg.StringContent()) {
			continue
		}
		if ts, _ := data["timestamp"].(string); ts != "" {
			msg.Timestamp = adapters.ParseTime(ts)
		}
		conv.Messages = append(conv.Messages, msg)
	}
	if conv.Title == "" {
		for _, m := range conv.Messages {
			if m.Role == models.RoleUser && m.Content != "" {
				conv.Title = adapters.Clip(m.Content, 80)
				break
			}
		}
	}
	return conv, scanner.Err()
}

func (a *Adapter) DefaultTarget(conv *models.Conversation) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cwd := adapters.CwdFromMeta(conv)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	id := adapters.NewUUID()
	dir := filepath.Join(home, ".claude", "projects", encodeClaudeProject(cwd))
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
	sessionID := strings.TrimSuffix(filepath.Base(targetPath), ".jsonl")
	cwd := adapters.CwdFromMeta(conv)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	var parent string
	for i, msg := range conv.Messages {
		uuid := adapters.NewUUID()
		ts := time.Now().UTC().Format(time.RFC3339Nano)
		if msg.Timestamp != nil {
			ts = msg.Timestamp.UTC().Format(time.RFC3339Nano)
		}
		role := "assistant"
		typ := "assistant"
		if msg.Role == models.RoleUser {
			role = "user"
			typ = "user"
		}
		rec := map[string]interface{}{
			"type":      typ,
			"uuid":      uuid,
			"timestamp": ts,
			"cwd":       cwd,
			"sessionId": sessionID,
			"message": map[string]interface{}{
				"role":    role,
				"content": claudeContentBlocks(msg),
			},
		}
		if parent != "" {
			rec["parentUuid"] = parent
		}
		if i == 0 {
			rec["parentUuid"] = nil
		}
		if err := writeJSONLine(w, rec); err != nil {
			return "", err
		}
		parent = uuid
	}
	if conv.Title != "" {
		if err := writeJSONLine(w, map[string]interface{}{
			"type":      "ai-title",
			"aiTitle":   conv.Title,
			"sessionId": sessionID,
		}); err != nil {
			return "", err
		}
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	return targetPath, nil
}

func claudeParts(data map[string]interface{}) []models.Part {
	messageObj, ok := data["message"].(map[string]interface{})
	if !ok {
		return nil
	}
	switch content := messageObj["content"].(type) {
	case string:
		if strings.TrimSpace(content) == "" {
			return nil
		}
		return []models.Part{models.TextPart(content)}
	case []interface{}:
		var parts []models.Part
		for _, raw := range content {
			partMap, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			switch partMap["type"] {
			case "text":
				if text, _ := partMap["text"].(string); strings.TrimSpace(text) != "" {
					parts = append(parts, models.TextPart(text))
				}
			case "thinking":
				thought, _ := partMap["thinking"].(string)
				if thought == "" {
					thought, _ = partMap["text"].(string)
				}
				if strings.TrimSpace(thought) != "" {
					parts = append(parts, models.ThinkingPart(thought))
				}
			case "tool_use":
				id, _ := partMap["id"].(string)
				name, _ := partMap["name"].(string)
				args := marshalJSON(partMap["input"])
				parts = append(parts, models.ToolCallPart(id, name, args))
			case "tool_result":
				id, _ := partMap["tool_use_id"].(string)
				isErr, _ := partMap["is_error"].(bool)
				parts = append(parts, models.ToolResultPart(id, claudeResultText(partMap["content"]), isErr))
			}
		}
		return parts
	default:
		return nil
	}
}

func claudeResultText(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []interface{}:
		var b strings.Builder
		for _, item := range t {
			if m, ok := item.(map[string]interface{}); ok {
				if text, _ := m["text"].(string); text != "" {
					b.WriteString(text)
					b.WriteByte('\n')
				}
			}
		}
		return strings.TrimSpace(b.String())
	default:
		return ""
	}
}

func claudeContentBlocks(msg models.Message) []map[string]interface{} {
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

func marshalJSON(v interface{}) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func encodeClaudeProject(cwd string) string {
	cwd = filepath.ToSlash(filepath.Clean(cwd))
	s := strings.ReplaceAll(cwd, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	if !strings.HasPrefix(s, "-") {
		s = "-" + s
	}
	return s
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
