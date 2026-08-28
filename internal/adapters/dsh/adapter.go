package dsh

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func dataRoot() string {
	if env := os.Getenv("DSH_HOME"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dsh")
}

func (a *Adapter) ListConversations() ([]adapters.ChatInfo, error) {
	root := filepath.Join(dataRoot(), "sessions")
	var chats []adapters.ChatInfo
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := info.Name()
		if base != "session.jsonl" && base != "session.jsonl.zstd" {
			return nil
		}
		chats = append(chats, summarizeLog(path, info.ModTime()))
		return nil
	})
	return chats, nil
}

func summarizeLog(path string, mod time.Time) adapters.ChatInfo {
	id := filepath.Base(filepath.Dir(path))
	info := adapters.ChatInfo{ID: id, Path: path, Agent: "dsh", UpdatedAt: mod, Title: "Untitled"}
	if conv, err := extractLog(path); err == nil {
		if conv.Title != "" {
			info.Title = conv.Title
		}
		if cwd := adapters.CwdFromMeta(conv); cwd != "" {
			info.Project = adapters.ShortPath(cwd)
		}
		if conv.ID != "" {
			info.ID = conv.ID
		}
	}
	return info
}

func (a *Adapter) Extract(sourcePath string) (*models.Conversation, error) {
	return extractLog(sourcePath)
}

func extractLog(path string) (*models.Conversation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(path, ".zstd") || strings.HasSuffix(path, ".zst") {
		dec, err := zstd.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("zstd: %w", err)
		}
		data, err = io.ReadAll(dec)
		dec.Close()
		if err != nil {
			return nil, fmt.Errorf("zstd: %w", err)
		}
	}

	conv := &models.Conversation{
		AgentSource: "dsh",
		Messages:    []models.Message{},
		Metadata:    map[string]interface{}{},
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec map[string]interface{}
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		typ, _ := rec["type"].(string)
		if first && typ == "session" {
			if id, _ := rec["id"].(string); id != "" {
				conv.ID = id
			}
			if cwd, _ := rec["cwd"].(string); cwd != "" {
				conv.Metadata["cwd"] = cwd
			}
			first = false
			continue
		}
		first = false
		ts := eventTime(rec)
		switch typ {
		case "user/message", "system/message":
			text := dshText(rec["data"])
			if text == "" {
				continue
			}
			role := models.RoleUser
			if typ == "system/message" {
				role = models.RoleSystem
			}
			conv.Messages = append(conv.Messages, models.NewMessage(role, []models.Part{models.TextPart(text)}))
			if ts != nil {
				conv.Messages[len(conv.Messages)-1].Timestamp = ts
			}
		case "assistant/message":
			data := asMap(rec["data"])
			text := dshText(data)
			if text == "" && data != nil {
				text = dshText(data["message"])
			}
			if text == "" {
				continue
			}
			msg := models.NewMessage(models.RoleAssistant, []models.Part{models.TextPart(text)})
			msg.Timestamp = ts
			conv.Messages = append(conv.Messages, msg)
		}
	}
	if conv.ID == "" {
		conv.ID = strings.TrimSuffix(filepath.Base(filepath.Dir(path)), "")
		if conv.ID == "." || conv.ID == "" {
			conv.ID = filepath.Base(path)
		}
	}
	for _, m := range conv.Messages {
		if m.Role == models.RoleUser {
			conv.Title = adapters.Clip(m.Content, 80)
			break
		}
	}
	return conv, sc.Err()
}

func eventTime(rec map[string]interface{}) *time.Time {
	switch v := rec["time"].(type) {
	case float64:
		return adapters.UnixMillisPtr(int64(v))
	}
	return nil
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func dshText(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]interface{}:
		if s, ok := t["content"].(string); ok {
			return strings.TrimSpace(s)
		}
		if s, ok := t["text"].(string); ok {
			return strings.TrimSpace(s)
		}
		if inner, ok := t["message"]; ok {
			return dshText(inner)
		}
		if parts, ok := t["content"].([]interface{}); ok {
			var b strings.Builder
			for _, p := range parts {
				b.WriteString(dshText(p))
				b.WriteString("\n")
			}
			return strings.TrimSpace(b.String())
		}
	}
	return ""
}

func (a *Adapter) DefaultTarget(conv *models.Conversation) (string, error) {
	cwd := adapters.CwdFromMeta(conv)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	ws := workspaceKey(cwd)
	sid := adapters.NewPrefixedID("ses_")
	return filepath.Join(dataRoot(), "sessions", ws, sid, "session.jsonl"), nil
}

func workspaceKey(cwd string) string {
	s := filepath.ToSlash(filepath.Clean(cwd))
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "workspace"
	}
	return s
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
	now := time.Now().UnixMilli()
	sid := filepath.Base(filepath.Dir(targetPath))
	cwd := adapters.CwdFromMeta(conv)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if err := writeJSONLine(w, map[string]interface{}{
		"type":            "session",
		"version":         1,
		"id":              sid,
		"cwd":             cwd,
		"createdAt":       now,
		"delegationDepth": 0,
		"origin":          "vibeporter",
	}); err != nil {
		return "", err
	}
	seq := 0
	for _, msg := range conv.Messages {
		ms := now
		if msg.Timestamp != nil {
			ms = msg.Timestamp.UnixMilli()
		}
		var rec map[string]interface{}
		switch msg.Role {
		case models.RoleUser:
			rec = map[string]interface{}{
				"type":      "user/message",
				"seq":       seq,
				"time":      ms,
				"surfaceOp": "add",
				"data": map[string]interface{}{
					"content": msg.StringContent(),
					"source":  map[string]string{"kind": "user"},
				},
			}
		case models.RoleSystem:
			rec = map[string]interface{}{
				"type":      "system/message",
				"seq":       seq,
				"time":      ms,
				"surfaceOp": "add",
				"data": map[string]interface{}{
					"content": msg.StringContent(),
					"source":  map[string]string{"kind": "system"},
				},
			}
		default:
			rec = map[string]interface{}{
				"type":      "assistant/message",
				"seq":       seq,
				"time":      ms,
				"surfaceOp": "add",
				"data": map[string]interface{}{
					"turn": 0,
					"step": 0,
					"message": map[string]interface{}{
						"content": msg.StringContent(),
					},
				},
			}
		}
		if err := writeJSONLine(w, rec); err != nil {
			return "", err
		}
		seq++
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	return targetPath, nil
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
