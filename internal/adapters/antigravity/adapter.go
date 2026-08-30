package antigravity

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

func brainRoot() string {
	if env := os.Getenv("ANTIGRAVITY_BRAIN_DIR"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gemini", "antigravity", "brain")
}

func (a *Adapter) ListConversations() ([]adapters.ChatInfo, error) {
	root := brainRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var chats []adapters.ChatInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		// transcript.jsonl under .system_generated/logs/
		transcript := filepath.Join(root, id, ".system_generated", "logs", "transcript.jsonl")
		if _, err := os.Stat(transcript); err != nil {
			// fallback: transcript_full
			transcript = filepath.Join(root, id, ".system_generated", "logs", "transcript_full.jsonl")
			if _, err := os.Stat(transcript); err != nil {
				continue
			}
		}
		info, err := os.Stat(transcript)
		if err != nil {
			continue
		}
		chat := summarizeTranscript(transcript, info.ModTime(), id)
		chats = append(chats, chat)
	}
	return chats, nil
}

func summarizeTranscript(path string, mod time.Time, fallbackID string) adapters.ChatInfo {
	info := adapters.ChatInfo{
		ID:        fallbackID,
		Path:      path,
		Agent:     "antigravity",
		UpdatedAt: mod,
		Title:     "Untitled",
	}
	// Try to get title from first user message
	_ = adapters.ForEachJSONLLimited(path, 512*1024, 256*1024, func(rec map[string]interface{}) {
		if info.Title != "Untitled" {
			return
		}
		source, _ := rec["source"].(string)
		typ, _ := rec["type"].(string)
		if source == "USER_EXPLICIT" && typ == "USER_INPUT" {
			content, _ := rec["content"].(string)
			txt := extractUserRequest(content)
			if txt != "" {
				info.Title = adapters.Clip(txt, 80)
			}
		}
	})
	// Project from brain folder's metadata? Try to infer from first tool call path
	// Fallback to brain ID
	if info.Project == "" {
		info.Project = fallbackID[:8]
	}
	return info
}

func extractUserRequest(content string) string {
	// Content like "<USER_REQUEST>\n...actual...\n</USER_REQUEST>\n<ADDITIONAL_METADATA>..."
	start := strings.Index(content, "<USER_REQUEST>")
	end := strings.Index(content, "</USER_REQUEST>")
	if start >= 0 && end > start {
		inner := content[start+len("<USER_REQUEST>") : end]
		return strings.TrimSpace(inner)
	}
	// fallback: first line
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "<") {
			return l
		}
	}
	return strings.TrimSpace(content)
}

func (a *Adapter) Extract(sourcePath string) (*models.Conversation, error) {
	// sourcePath may be ID or full path
	path := sourcePath
	if _, err := os.Stat(path); err != nil {
		// try as ID under brainRoot
		candidate := filepath.Join(brainRoot(), sourcePath, ".system_generated", "logs", "transcript.jsonl")
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
		} else {
			candidate = filepath.Join(brainRoot(), sourcePath, ".system_generated", "logs", "transcript_full.jsonl")
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			} else {
				return nil, fmt.Errorf("could not open file: %w", err)
			}
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	id := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	if id == "logs" || id == "." {
		id = strings.TrimSuffix(filepath.Base(path), ".jsonl")
		id = strings.TrimSuffix(id, ".json")
		if id == "transcript" || id == "transcript_full" {
			id = filepath.Base(filepath.Dir(filepath.Dir(path)))
		}
	}
	// If path is direct transcript, derive ID from brain folder
	if strings.Contains(path, "/brain/") {
		parts := strings.Split(filepath.ToSlash(path), "/brain/")
		if len(parts) > 1 {
			rest := parts[1]
			if idx := strings.Index(rest, "/"); idx > 0 {
				id = rest[:idx]
			}
		}
	}

	conv := &models.Conversation{
		ID:          id,
		AgentSource: "antigravity",
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
		source, _ := rec["source"].(string)
		typ, _ := rec["type"].(string)
		created, _ := rec["created_at"].(string)
		var ts *time.Time
		if created != "" {
			if parsed, err := time.Parse(time.RFC3339, created); err == nil {
				ts = &parsed
			} else if parsed, err := time.Parse(time.RFC3339Nano, created); err == nil {
				ts = &parsed
			}
		}

		switch source {
		case "USER_EXPLICIT":
			if typ == "USER_INPUT" {
				content, _ := rec["content"].(string)
				txt := extractUserRequest(content)
				if txt == "" {
					txt = strings.TrimSpace(content)
				}
				if txt == "" {
					continue
				}
				// Check for cwd in content? Not needed for now
				msg := models.NewMessage(models.RoleUser, []models.Part{models.TextPart(txt)})
				msg.Timestamp = ts
				conv.Messages = append(conv.Messages, msg)
				if conv.Title == "" {
					conv.Title = adapters.Clip(txt, 80)
				}
			}
		case "SYSTEM":
			if typ == "CHECKPOINT" {
				// Skip checkpoint summary noise, but keep as system if needed
				continue
			}
			content, _ := rec["content"].(string)
			if strings.TrimSpace(content) == "" {
				continue
			}
			msg := models.NewMessage(models.RoleSystem, []models.Part{models.TextPart(content)})
			msg.Timestamp = ts
			conv.Messages = append(conv.Messages, msg)
		case "MODEL":
			// PLANNER_RESPONSE, GENERIC, etc.
			var parts []models.Part
			if th, ok := rec["thinking"].(string); ok && strings.TrimSpace(th) != "" {
				parts = append(parts, models.ThinkingPart(th))
			}
			// content field (for GENERIC tool outputs)
			if content, ok := rec["content"].(string); ok && strings.TrimSpace(content) != "" {
				// Heuristic: if this is a tool output (contains Created At/Output), treat as text
				// For now just text part
				parts = append(parts, models.TextPart(content))
			}
			// tool_calls
			if raw, ok := rec["tool_calls"]; ok {
				if arr, ok := raw.([]interface{}); ok {
					for _, item := range arr {
						m, ok := item.(map[string]interface{})
						if !ok {
							continue
						}
						name, _ := m["name"].(string)
						if name == "" {
							continue
						}
						var argsJSON string
						if args, ok := m["args"]; ok {
							b, _ := json.Marshal(args)
							argsJSON = string(b)
						}
						parts = append(parts, models.ToolCallPart("", name, argsJSON))
					}
				}
			}
			if len(parts) == 0 {
				continue
			}
			msg := models.NewMessage(models.RoleAssistant, parts)
			msg.Timestamp = ts
			conv.Messages = append(conv.Messages, msg)
		default:
			// Unknown source — skip
			continue
		}
	}
	if conv.Title == "" {
		for _, m := range conv.Messages {
			if m.Role == models.RoleUser && m.Content != "" {
				conv.Title = adapters.Clip(m.Content, 80)
				break
			}
		}
	}
	return conv, sc.Err()
}

func (a *Adapter) DefaultTarget(conv *models.Conversation) (string, error) {
	// Write to a temp brain-like location: use brainRoot with new UUID
	id := adapters.NewUUID()
	root := brainRoot()
	return filepath.Join(root, id, ".system_generated", "logs", "transcript.jsonl"), nil
}

func (a *Adapter) Inject(conv *models.Conversation, targetPath string) (string, error) {
	var err error
	if strings.TrimSpace(targetPath) == "" {
		targetPath, err = a.DefaultTarget(conv)
		if err != nil {
			return "", err
		}
	}
	// If target is a directory or brain ID, construct transcript path
	if info, err := os.Stat(targetPath); err == nil && info.IsDir() {
		targetPath = filepath.Join(targetPath, ".system_generated", "logs", "transcript.jsonl")
	} else if !strings.HasSuffix(targetPath, ".jsonl") {
		// Assume it's a brain ID
		if !strings.Contains(targetPath, "/") {
			targetPath = filepath.Join(brainRoot(), targetPath, ".system_generated", "logs", "transcript.jsonl")
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
	for idx, msg := range conv.Messages {
		rec := map[string]interface{}{
			"step_index": idx,
			"created_at": time.Now().UTC().Format(time.RFC3339),
			"status":     "DONE",
		}
		switch msg.Role {
		case models.RoleUser:
			rec["source"] = "USER_EXPLICIT"
			rec["type"] = "USER_INPUT"
			rec["content"] = "<USER_REQUEST>\n" + msg.StringContent() + "\n</USER_REQUEST>"
		case models.RoleSystem:
			rec["source"] = "SYSTEM"
			rec["type"] = "CHECKPOINT"
			rec["content"] = msg.StringContent()
		case models.RoleAssistant:
			rec["source"] = "MODEL"
			// Distinguish thinking vs generic
			hasThinking := false
			hasTool := false
			for _, p := range msg.EffectiveParts() {
				if p.Kind == models.PartThinking {
					hasThinking = true
				}
				if p.Kind == models.PartToolCall {
					hasTool = true
				}
			}
			if hasThinking && !hasTool && len(msg.EffectiveParts()) == 1 {
				rec["type"] = "PLANNER_RESPONSE"
				for _, p := range msg.EffectiveParts() {
					if p.Kind == models.PartThinking {
						rec["thinking"] = p.Text
					}
				}
			} else if hasTool {
				rec["type"] = "PLANNER_RESPONSE"
				var thinking string
				var toolCalls []map[string]interface{}
				for _, p := range msg.EffectiveParts() {
					switch p.Kind {
					case models.PartThinking:
						thinking += p.Text + "\n"
					case models.PartToolCall:
						var args interface{} = map[string]interface{}{}
						if strings.TrimSpace(p.ArgsJSON) != "" {
							_ = json.Unmarshal([]byte(p.ArgsJSON), &args)
						}
						toolCalls = append(toolCalls, map[string]interface{}{"name": p.Name, "args": args})
					case models.PartText:
						rec["content"] = p.Text
					}
				}
				if thinking != "" {
					rec["thinking"] = strings.TrimSpace(thinking)
				}
				if len(toolCalls) > 0 {
					rec["tool_calls"] = toolCalls
				}
			} else {
				rec["type"] = "GENERIC"
				rec["content"] = msg.StringContent()
			}
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
