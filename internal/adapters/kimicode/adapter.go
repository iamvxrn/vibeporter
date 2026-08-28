package kimicode

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func dataRoot() string {
	if env := os.Getenv("KIMI_CODE_HOME"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kimi-code")
}

func (a *Adapter) ListConversations() ([]adapters.ChatInfo, error) {
	root := filepath.Join(dataRoot(), "sessions")
	var chats []adapters.ChatInfo
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Base(path) != "wire.jsonl" {
			return nil
		}
		slash := filepath.ToSlash(path)
		if !strings.Contains(slash, "/agents/main/") {
			return nil
		}
		chats = append(chats, summarizeSession(path, info.ModTime()))
		return nil
	})
	return chats, nil
}

func summarizeSession(wirePath string, mod time.Time) adapters.ChatInfo {
	sessionDir := filepath.Dir(filepath.Dir(filepath.Dir(wirePath))) // .../sessionId
	id := filepath.Base(sessionDir)
	info := adapters.ChatInfo{ID: id, Path: wirePath, Agent: "kimicode", UpdatedAt: mod, Title: "Untitled"}
	if st := readState(filepath.Join(sessionDir, "state.json")); st != nil {
		if st.Title != "" {
			info.Title = st.Title
		}
		info.Project = adapters.ShortPath(st.WorkDir)
		if st.UpdatedAt > 0 {
			if t := adapters.UnixMillisPtr(st.UpdatedAt); t != nil {
				info.UpdatedAt = *t
			}
		}
	}
	if info.Title == "Untitled" {
		if conv, err := extractWire(wirePath); err == nil && conv.Title != "" {
			info.Title = conv.Title
		}
	}
	return info
}

type stateFile struct {
	Title      string `json:"title"`
	WorkDir    string `json:"workDir"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
	LastPrompt string `json:"lastPrompt"`
}

func readState(path string) *stateFile {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var st stateFile
	if json.Unmarshal(b, &st) != nil {
		return nil
	}
	return &st
}

func (a *Adapter) Extract(sourcePath string) (*models.Conversation, error) {
	wire := sourcePath
	if st, err := os.Stat(sourcePath); err == nil && st.IsDir() {
		wire = filepath.Join(sourcePath, "agents", "main", "wire.jsonl")
	}
	conv, err := extractWire(wire)
	if err != nil {
		return nil, err
	}
	sessionDir := filepath.Dir(filepath.Dir(filepath.Dir(wire)))
	if st := readState(filepath.Join(sessionDir, "state.json")); st != nil {
		if st.Title != "" {
			conv.Title = st.Title
		}
		if st.WorkDir != "" {
			adapters.EnsureMeta(conv)["cwd"] = st.WorkDir
		}
	}
	return conv, nil
}

func extractWire(path string) (*models.Conversation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	id := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	conv := &models.Conversation{
		ID:          id,
		AgentSource: "kimicode",
		Messages:    []models.Message{},
		Metadata:    map[string]interface{}{},
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
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
		ts := wireTime(rec)
		switch typ {
		case "turn.prompt":
			kind := "user"
			if origin, _ := rec["origin"].(map[string]interface{}); origin != nil {
				if k, _ := origin["kind"].(string); k != "" {
					kind = k
				}
			}
			role := models.RoleUser
			switch kind {
			case "user":
				role = models.RoleUser
			case "system":
				role = models.RoleSystem
			default:
				continue
			}
			text := kimiInputText(rec["input"])
			if text == "" {
				continue
			}
			msg := models.NewMessage(role, []models.Part{models.TextPart(text)})
			msg.Timestamp = ts
			conv.Messages = append(conv.Messages, msg)
		case "context.append_loop_event":
			ev := asMap(rec["event"])
			if ev == nil {
				continue
			}
			switch ev["type"] {
			case "content.part":
				part := asMap(ev["part"])
				if part == nil {
					continue
				}
				switch part["type"] {
				case "text":
					text, _ := part["text"].(string)
					text = strings.TrimSpace(text)
					if text == "" {
						continue
					}
					appendAssistantPart(conv, models.TextPart(text), ts)
				case "thinking", "reasoning":
					text, _ := part["text"].(string)
					if strings.TrimSpace(text) != "" {
						appendAssistantPart(conv, models.ThinkingPart(text), ts)
					}
				}
			case "tool.call":
				name, _ := ev["name"].(string)
				if name == "" {
					continue
				}
				args := ""
				if in := ev["input"]; in != nil {
					b, _ := json.Marshal(in)
					args = string(b)
				} else if in := ev["arguments"]; in != nil {
					b, _ := json.Marshal(in)
					args = string(b)
				}
				appendAssistantPart(conv, models.ToolCallPart("", name, args), ts)
			case "tool.result":
				text := kimiInputText(ev["output"])
				if text == "" {
					text = kimiInputText(ev["result"])
				}
				id, _ := ev["id"].(string)
				if id == "" {
					id, _ = ev["tool_call_id"].(string)
				}
				appendAssistantPart(conv, models.ToolResultPart(id, text, false), ts)
			}
		}
	}
	if conv.Title == "" {
		for _, m := range conv.Messages {
			if m.Role == models.RoleUser {
				conv.Title = adapters.Clip(m.Content, 80)
				break
			}
		}
	}
	return conv, sc.Err()
}

func appendAssistantPart(conv *models.Conversation, p models.Part, ts *time.Time) {
	n := len(conv.Messages)
	if n > 0 && conv.Messages[n-1].Role == models.RoleAssistant {
		conv.Messages[n-1].Parts = append(conv.Messages[n-1].Parts, p)
		conv.Messages[n-1].SyncContent()
		if ts != nil {
			conv.Messages[n-1].Timestamp = ts
		}
		return
	}
	msg := models.NewMessage(models.RoleAssistant, []models.Part{p})
	msg.Timestamp = ts
	conv.Messages = append(conv.Messages, msg)
}

func asMap(v interface{}) map[string]interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return t
	case string:
		var m map[string]interface{}
		if json.Unmarshal([]byte(t), &m) == nil {
			return m
		}
	}
	return nil
}

func kimiInputText(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []interface{}:
		var b strings.Builder
		for _, p := range t {
			m, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			if txt, ok := m["text"].(string); ok {
				b.WriteString(txt)
			}
		}
		return strings.TrimSpace(b.String())
	case map[string]interface{}:
		if txt, ok := t["text"].(string); ok {
			return strings.TrimSpace(txt)
		}
	}
	return ""
}

func wireTime(rec map[string]interface{}) *time.Time {
	switch v := rec["time"].(type) {
	case float64:
		return adapters.UnixMillisPtr(int64(v))
	}
	if s, ok := rec["created_at"].(string); ok {
		return adapters.ParseTime(s)
	}
	if n, ok := rec["createdAt"].(float64); ok {
		return adapters.UnixMillisPtr(int64(n))
	}
	return nil
}

func (a *Adapter) DefaultTarget(conv *models.Conversation) (string, error) {
	cwd := adapters.CwdFromMeta(conv)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	sum := sha256.Sum256([]byte(cwd))
	wdKey := "wd_vp_" + hex.EncodeToString(sum[:])[:10]
	sid := "session_" + strings.ReplaceAll(adapters.NewUUID(), "-", "")
	return filepath.Join(dataRoot(), "sessions", wdKey, sid, "agents", "main", "wire.jsonl"), nil
}

func (a *Adapter) Inject(conv *models.Conversation, targetPath string) (string, error) {
	var err error
	if strings.TrimSpace(targetPath) == "" {
		targetPath, err = a.DefaultTarget(conv)
		if err != nil {
			return "", err
		}
	}
	if filepath.Base(targetPath) != "wire.jsonl" {
		targetPath = filepath.Join(targetPath, "agents", "main", "wire.jsonl")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}

	sessionDir := filepath.Dir(filepath.Dir(filepath.Dir(targetPath)))
	sessionID := filepath.Base(sessionDir)
	cwd := adapters.CwdFromMeta(conv)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	now := time.Now().UnixMilli()
	title := conv.Title
	if title == "" {
		title = "Imported session"
	}

	state, _ := json.Marshal(map[string]interface{}{
		"title":     title,
		"workDir":   cwd,
		"createdAt": now,
		"updatedAt": now,
	})
	if err := os.WriteFile(filepath.Join(sessionDir, "state.json"), state, 0o644); err != nil {
		return "", err
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	w := bufio.NewWriter(f)
	_ = writeJSONLine(w, map[string]interface{}{
		"type":             "metadata",
		"protocol_version": 1,
		"created_at":       now,
	})
	for _, msg := range conv.Messages {
		ms := now
		if msg.Timestamp != nil {
			ms = msg.Timestamp.UnixMilli()
		}
		if msg.Role == models.RoleUser || msg.Role == models.RoleSystem {
			kind := "user"
			if msg.Role == models.RoleSystem {
				kind = "system"
			}
			_ = writeJSONLine(w, map[string]interface{}{
				"type":   "turn.prompt",
				"time":   ms,
				"origin": map[string]string{"kind": kind},
				"input":  msg.StringContent(),
			})
			continue
		}
		for _, p := range msg.EffectiveParts() {
			var ev map[string]interface{}
			switch p.Kind {
			case models.PartThinking:
				ev = map[string]interface{}{
					"type": "content.part",
					"part": map[string]string{"type": "thinking", "text": p.Text},
				}
			case models.PartToolCall:
				var input interface{} = map[string]interface{}{}
				if strings.TrimSpace(p.ArgsJSON) != "" {
					_ = json.Unmarshal([]byte(p.ArgsJSON), &input)
				}
				ev = map[string]interface{}{"type": "tool.call", "name": p.Name, "input": input}
			case models.PartToolResult:
				ev = map[string]interface{}{"type": "tool.result", "id": p.ToolCallID, "output": p.Text}
			default:
				ev = map[string]interface{}{
					"type": "content.part",
					"part": map[string]string{"type": "text", "text": p.Text},
				}
			}
			_ = writeJSONLine(w, map[string]interface{}{
				"type":  "context.append_loop_event",
				"time":  ms,
				"event": ev,
			})
		}
	}
	if err := w.Flush(); err != nil {
		return "", err
	}

	idxPath := filepath.Join(dataRoot(), "session_index.jsonl")
	idx, err := os.OpenFile(idxPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		line, _ := json.Marshal(map[string]string{
			"sessionId":  sessionID,
			"sessionDir": sessionDir,
			"workDir":    cwd,
		})
		_, _ = idx.Write(append(line, '\n'))
		_ = idx.Close()
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
