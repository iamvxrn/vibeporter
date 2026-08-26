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
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	conv := &models.Conversation{
		AgentSource: "gemini",
		Messages:    []models.Message{},
	}

	// Rebuild the conversation the way Gemini CLI does: keep messages in
	// insertion order, de-dupe by id (last-wins), and honor $rewindTo.
	order := []string{}
	byID := map[string]map[string]interface{}{}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // sessions can have long lines
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}

		var rec map[string]interface{}
		if err := json.Unmarshal(line, &rec); err != nil {
			continue // skip malformed lines
		}

		if rewindTo, ok := rec["$rewindTo"].(string); ok {
			order = applyRewind(order, byID, rewindTo)
			continue
		}
		if id, ok := rec["id"].(string); ok {
			if _, seen := byID[id]; !seen {
				order = append(order, id)
			}
			byID[id] = rec
			continue
		}
		// Metadata line ({sessionId, projectHash, ...}) or a $set update.
		if sid, ok := rec["sessionId"].(string); ok && conv.ID == "" {
			conv.ID = sid
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
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

		text := extractText(rec["content"])
		if toolCalls := extractToolCalls(rec["toolCalls"]); toolCalls != "" {
			if text != "" {
				text += "\n"
			}
			text += toolCalls
		}
		if strings.TrimSpace(text) == "" {
			continue
		}

		msg := models.Message{Role: role, Content: strings.TrimSpace(text)}
		if ts, ok := rec["timestamp"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
				msg.Timestamp = &parsed
			}
		}
		conv.Messages = append(conv.Messages, msg)
	}

	return conv, nil
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

// extractToolCalls renders the `toolCalls` array a gemini message may carry into
// human-readable markers, so tool activity survives the migration.
func extractToolCalls(v interface{}) string {
	arr, ok := v.([]interface{})
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := m["name"].(string); ok {
			b.WriteString(fmt.Sprintf("[Tool Use: %s]\n", name))
		}
	}
	return b.String()
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

func (a *Adapter) Inject(conv *models.Conversation, targetPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}

	file, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

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
		rec := map[string]interface{}{
			"id":        newUUID(),
			"timestamp": now,
			"type":      recType,
			"content":   []map[string]string{{"text": msg.Content}},
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
