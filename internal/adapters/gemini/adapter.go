// Package gemini adapts Gemini CLI chat sessions to and from Vibeporter's
// intermediate representation.
//
// Gemini CLI records each session as an append-only JSON Lines file under
// ~/.gemini/tmp/<project_hash>/chats/session-*.jsonl (older versions used a
// monolithic .json file). The first line holds session metadata; the rest are
// message records ({id, timestamp, type, content, ...}), $set metadata updates,
// and $rewindTo markers. Because a message line is re-appended as its content
// or tool calls grow, records are de-duplicated by id with last-write-wins.
package gemini

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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

// messageRole maps a Gemini CLI record type to an IR role. The bool reports
// whether the record is part of the conversation proper — "info" and "error"
// records are UI notices and are dropped.
func messageRole(recordType string) (models.Role, bool) {
	switch recordType {
	case "user":
		return models.RoleUser, true
	case "gemini":
		return models.RoleAssistant, true
	default:
		return models.RoleSystem, false
	}
}

func (a *Adapter) Extract(sourcePath string) (*models.Conversation, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}

	conv := &models.Conversation{
		AgentSource: "gemini",
		Messages:    []models.Message{},
	}

	// Rebuild the conversation the way Gemini CLI does: keep messages in
	// insertion order, de-dupe by id (last-wins), and honor $rewindTo.
	order := []string{}
	byID := map[string]map[string]interface{}{}

	if isMonolithicJSON(sourcePath) {
		ingestJSONFile(data, conv, &order, byID)
	} else {
		if err := ingestJSONL(data, conv, &order, byID); err != nil {
			return nil, err
		}
	}

	if conv.ID == "" {
		conv.ID = strings.TrimSuffix(strings.TrimSuffix(filepath.Base(sourcePath), ".jsonl"), ".json")
	}

	for _, id := range order {
		rec := byID[id]
		recType, _ := rec["type"].(string)
		role, keep := messageRole(recType)
		if !keep {
			continue
		}
		parts := geminiParts(rec)
		if len(parts) == 0 {
			continue
		}
		msg := models.NewMessage(role, parts)
		if ts, ok := rec["timestamp"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
				msg.Timestamp = &parsed
			}
		}
		conv.Messages = append(conv.Messages, msg)
	}

	return conv, nil
}

func isMonolithicJSON(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".json") && !strings.HasSuffix(lower, ".jsonl")
}

func ingestRecord(rec map[string]interface{}, conv *models.Conversation, order *[]string, byID map[string]map[string]interface{}) {
	if rewindTo, ok := rec["$rewindTo"].(string); ok {
		*order = applyRewind(*order, byID, rewindTo)
		return
	}
	if id, ok := rec["id"].(string); ok {
		if _, seen := byID[id]; !seen {
			*order = append(*order, id)
		}
		byID[id] = rec
		return
	}
	if sid, ok := rec["sessionId"].(string); ok && conv.ID == "" {
		conv.ID = sid
	}
}

func ingestJSONL(data []byte, conv *models.Conversation, order *[]string, byID map[string]map[string]interface{}) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec map[string]interface{}
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		ingestRecord(rec, conv, order, byID)
	}
	return scanner.Err()
}

func ingestJSONFile(data []byte, conv *models.Conversation, order *[]string, byID map[string]map[string]interface{}) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return
	}
	if trimmed[0] == '[' {
		var recs []map[string]interface{}
		if err := json.Unmarshal(trimmed, &recs); err != nil {
			return
		}
		for _, rec := range recs {
			ingestRecord(rec, conv, order, byID)
		}
		return
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return
	}
	if sid, ok := obj["sessionId"].(string); ok && conv.ID == "" {
		conv.ID = sid
	}
	if raw, ok := obj["messages"]; ok {
		if msgs, ok := raw.([]interface{}); ok {
			for _, m := range msgs {
				if rec, ok := m.(map[string]interface{}); ok {
					ingestRecord(rec, conv, order, byID)
				}
			}
		}
		return
	}
	ingestRecord(obj, conv, order, byID)
}

// applyRewind drops the target message and everything after it, mirroring
// Gemini CLI's $rewindTo semantics. An unknown target clears the transcript.
func applyRewind(order []string, byID map[string]map[string]interface{}, rewindTo string) []string {
	idx := -1
	for i, id := range order {
		if id == rewindTo {
			idx = i
			break
		}
	}
	if idx == -1 {
		for k := range byID {
			delete(byID, k)
		}
		return order[:0]
	}
	for _, id := range order[idx:] {
		delete(byID, id)
	}
	return order[:idx]
}

// extractText flattens a Gemini CLI `content` field, which is a PartListUnion:
// a plain string, a single Part object, or an array of Parts.
func extractText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var b strings.Builder
		for _, p := range v {
			b.WriteString(partText(p))
		}
		return b.String()
	case map[string]interface{}:
		return partText(v)
	default:
		return ""
	}
}

func partText(part interface{}) string {
	m, ok := part.(map[string]interface{})
	if !ok {
		return ""
	}
	if text, ok := m["text"].(string); ok {
		return text
	}
	if fc, ok := m["functionCall"].(map[string]interface{}); ok {
		name, _ := fc["name"].(string)
		return fmt.Sprintf("[Tool Call: %s]\n", name)
	}
	if _, ok := m["functionResponse"].(map[string]interface{}); ok {
		return "[Tool Result]\n"
	}
	return ""
}

func geminiParts(rec map[string]interface{}) []models.Part {
	var parts []models.Part
	switch th := rec["thoughts"].(type) {
	case string:
		if strings.TrimSpace(th) != "" {
			parts = append(parts, models.ThinkingPart(th))
		}
	case []interface{}:
		for _, item := range th {
			if m, ok := item.(map[string]interface{}); ok {
				if t, _ := m["text"].(string); strings.TrimSpace(t) != "" {
					parts = append(parts, models.ThinkingPart(t))
				}
			}
		}
	}
	switch v := rec["content"].(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			parts = append(parts, models.TextPart(v))
		}
	case []interface{}:
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := m["text"].(string); ok && strings.TrimSpace(text) != "" {
				parts = append(parts, models.TextPart(text))
				continue
			}
			if fc, ok := m["functionCall"].(map[string]interface{}); ok {
				name, _ := fc["name"].(string)
				args := ""
				if in := fc["args"]; in != nil {
					b, _ := json.Marshal(in)
					args = string(b)
				} else if in := fc["arguments"]; in != nil {
					b, _ := json.Marshal(in)
					args = string(b)
				}
				parts = append(parts, models.ToolCallPart("", name, args))
				continue
			}
			if fr, ok := m["functionResponse"].(map[string]interface{}); ok {
				text := ""
				if s, ok := fr["response"].(string); ok {
					text = s
				} else if s, ok := fr["output"].(string); ok {
					text = s
				} else if fr["response"] != nil {
					b, _ := json.Marshal(fr["response"])
					text = string(b)
				}
				parts = append(parts, models.ToolResultPart("", text, false))
			}
		}
	}
	hasCall := false
	for _, p := range parts {
		if p.Kind == models.PartToolCall {
			hasCall = true
			break
		}
	}
	if !hasCall {
		if arr, ok := rec["toolCalls"].([]interface{}); ok {
			for _, item := range arr {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := m["name"].(string)
				if name == "" {
					continue
				}
				args := ""
				if in := m["args"]; in != nil {
					b, _ := json.Marshal(in)
					args = string(b)
				}
				parts = append(parts, models.ToolCallPart("", name, args))
			}
		}
	}
	return parts
}

func geminiContent(msg models.Message) []map[string]interface{} {
	var out []map[string]interface{}
	for _, p := range msg.EffectiveParts() {
		switch p.Kind {
		case models.PartText:
			out = append(out, map[string]interface{}{"text": p.Text})
		case models.PartToolCall:
			var args interface{} = map[string]interface{}{}
			if strings.TrimSpace(p.ArgsJSON) != "" {
				_ = json.Unmarshal([]byte(p.ArgsJSON), &args)
			}
			out = append(out, map[string]interface{}{
				"functionCall": map[string]interface{}{"name": p.Name, "args": args},
			})
		case models.PartToolResult:
			out = append(out, map[string]interface{}{
				"functionResponse": map[string]interface{}{"name": p.Name, "output": p.Text},
			})
		}
	}
	if len(out) == 0 {
		out = []map[string]interface{}{{"text": msg.Content}}
	}
	return out
}

func geminiThoughts(msg models.Message) string {
	var b strings.Builder
	for _, p := range msg.EffectiveParts() {
		if p.Kind == models.PartThinking && strings.TrimSpace(p.Text) != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func geminiToolCalls(msg models.Message) []map[string]interface{} {
	var out []map[string]interface{}
	for _, p := range msg.EffectiveParts() {
		if p.Kind != models.PartToolCall {
			continue
		}
		item := map[string]interface{}{"name": p.Name}
		if strings.TrimSpace(p.ArgsJSON) != "" {
			var args interface{}
			if json.Unmarshal([]byte(p.ArgsJSON), &args) == nil {
				item["args"] = args
			}
		}
		out = append(out, item)
	}
	return out
}

func (a *Adapter) ListConversations() ([]adapters.ChatInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	tmpDir := filepath.Join(home, ".gemini", "tmp")

	var chats []adapters.ChatInfo
	walkErr := filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil // skip unreadable entries (including a missing tmp dir)
		}
		if info.IsDir() {
			return nil
		}
		// Session transcripts live under <project_hash>/chats/ and use .jsonl
		// (current) or .json (legacy).
		if !strings.Contains(filepath.ToSlash(path), "/chats/") {
			return nil
		}
		name := info.Name()
		if !strings.HasSuffix(name, ".jsonl") && !strings.HasSuffix(name, ".json") {
			return nil
		}
		id := strings.TrimSuffix(strings.TrimSuffix(name, ".jsonl"), ".json")
		chats = append(chats, summarizeGeminiFile(path, info.ModTime(), id))
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		return nil, walkErr
	}
	return chats, nil
}

func summarizeGeminiFile(path string, modTime time.Time, fallbackID string) adapters.ChatInfo {
	info := adapters.ChatInfo{
		ID:        fallbackID,
		Path:      path,
		Agent:     "gemini",
		UpdatedAt: modTime,
	}

	var firstUser string
	_ = adapters.ForEachJSONLLimited(path, 256*1024, 64*1024, func(rec map[string]interface{}) {
		if sid, ok := rec["sessionId"].(string); ok && sid != "" {
			info.ID = sid
		}
		for _, key := range []string{"lastUpdated", "startTime", "timestamp"} {
			ts, _ := rec[key].(string)
			if ts == "" {
				continue
			}
			if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
				info.UpdatedAt = parsed
				break
			}
			if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				info.UpdatedAt = parsed
				break
			}
		}
		if set, ok := rec["$set"].(map[string]interface{}); ok {
			if ts, _ := set["lastUpdated"].(string); ts != "" {
				if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
					info.UpdatedAt = parsed
				}
			}
		}
		if rec["type"] == "user" && firstUser == "" {
			firstUser = extractText(rec["content"])
		}
	})

	info.Title = adapters.Clip(firstUser, 80)
	if info.Title == "" {
		info.Title = "Untitled"
	}
	return info
}

func (a *Adapter) DefaultTarget(conv *models.Conversation) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	seed := adapters.CwdFromMeta(conv)
	if seed == "" {
		seed = conv.ID
	}
	name := fmt.Sprintf("session-%s-%s.jsonl", time.Now().UTC().Format("2006-01-02T15-04-05"), adapters.NewUUID()[:8])
	return filepath.Join(home, ".gemini", "tmp", projectHash(seed), "chats", name), nil
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

	file, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	w := bufio.NewWriter(file)
	now := time.Now().UTC().Format(time.RFC3339)

	// Gemini CLI recognizes a modern JSONL session by a first line carrying both
	// sessionId and projectHash.
	meta := map[string]interface{}{
		"sessionId":   newUUID(),
		"projectHash": projectHash(conv.ID),
		"startTime":   now,
		"lastUpdated": now,
		"kind":        "main",
	}
	if err := writeJSONLine(w, meta); err != nil {
		return "", err
	}

	for _, msg := range conv.Messages {
		recType := "gemini"
		if msg.Role == models.RoleUser {
			recType = "user"
		}
		ts := now
		if msg.Timestamp != nil {
			ts = msg.Timestamp.UTC().Format(time.RFC3339)
		}
		rec := map[string]interface{}{
			"id":        newUUID(),
			"timestamp": ts,
			"type":      recType,
			"content":   geminiContent(msg),
		}
		if thoughts := geminiThoughts(msg); thoughts != "" {
			rec["thoughts"] = thoughts
		}
		if calls := geminiToolCalls(msg); len(calls) > 0 {
			rec["toolCalls"] = calls
		}
		if err := writeJSONLine(w, rec); err != nil {
			return "", err
		}
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

// projectHash derives a stable 64-char hex id from a seed, matching the shape of
// Gemini CLI's own project hashes. The value is informational for imported files.
func projectHash(seed string) string {
	if seed == "" {
		seed = "vibeporter-import"
	}
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("vp-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
