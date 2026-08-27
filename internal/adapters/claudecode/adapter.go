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
		text := firstUserText(data)
		if text == "" {
			continue
		}
		if isSlashCommandDump(text) {
			continue
		}
		msg := models.Message{Role: role, Content: text}
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
				"content": []map[string]string{{"type": "text", "text": msg.Content}},
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

func encodeClaudeProject(cwd string) string {
	cwd = filepath.ToSlash(filepath.Clean(cwd))
	s := strings.ReplaceAll(cwd, "/", "-")
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
