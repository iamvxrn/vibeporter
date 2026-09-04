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

		// Check for a vibeporter-generated handoff markdown first.
		handoffMD := filepath.Join(root, id, "handoff_context.md")
		if info, err := os.Stat(handoffMD); err == nil {
			chat := adapters.ChatInfo{
				ID:        id,
				Path:      handoffMD,
				Agent:     "antigravity",
				UpdatedAt: info.ModTime(),
				Title:     titleFromHandoffMD(handoffMD),
			}
			if chat.Title == "" {
				chat.Title = "Handoff"
			}
			chat.Project = safePrefix(id, 8)
			chats = append(chats, chat)
			continue
		}

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

// titleFromHandoffMD reads the first H1 heading from a markdown file.
func titleFromHandoffMD(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "# ") {
			return adapters.Clip(strings.TrimPrefix(line, "# "), 80)
		}
	}
	return ""
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
	if info.Project == "" {
		info.Project = safePrefix(fallbackID, 8)
	}
	return info
}

// safePrefix returns the first n characters of s, or s itself if shorter.
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
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
	// sourcePath may be ID, full path to transcript.jsonl, or handoff_context.md
	path := sourcePath
	if _, err := os.Stat(path); err != nil {
		// try as ID under brainRoot
		// First check for handoff markdown
		candidate := filepath.Join(brainRoot(), sourcePath, "handoff_context.md")
		if _, err := os.Stat(candidate); err == nil {
			return extractHandoffMD(candidate, sourcePath)
		}
		candidate = filepath.Join(brainRoot(), sourcePath, ".system_generated", "logs", "transcript.jsonl")
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
		} else {
			candidate = filepath.Join(brainRoot(), sourcePath, ".system_generated", "logs", "transcript_full.jsonl")
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			} else {
				return nil, fmt.Errorf("could not find conversation %q: no transcript or handoff file", sourcePath)
			}
		}
	}

	// Check if path points to a handoff markdown
	if strings.HasSuffix(path, ".md") {
		id := deriveIDFromPath(path)
		return extractHandoffMD(path, id)
	}

	return extractTranscript(path)
}

// extractHandoffMD extracts a conversation from a vibeporter handoff markdown file.
func extractHandoffMD(path, id string) (*models.Conversation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read handoff file: %w", err)
	}
	content := string(data)
	conv := &models.Conversation{
		ID:          id,
		AgentSource: "antigravity",
		Messages:    []models.Message{},
		Metadata:    map[string]interface{}{},
	}
	// The handoff markdown is one big context block — treat it as a system message.
	msg := models.NewMessage(models.RoleSystem, []models.Part{models.TextPart(content)})
	conv.Messages = append(conv.Messages, msg)

	// Try to extract title from first heading
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "# ") {
			conv.Title = adapters.Clip(strings.TrimPrefix(l, "# "), 80)
			break
		}
	}
	if conv.Title == "" {
		conv.Title = "Handoff"
	}
	return conv, nil
}

func extractTranscript(path string) (*models.Conversation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	id := deriveIDFromPath(path)

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
				msg := models.NewMessage(models.RoleUser, []models.Part{models.TextPart(txt)})
				msg.Timestamp = ts
				conv.Messages = append(conv.Messages, msg)
				if conv.Title == "" {
					conv.Title = adapters.Clip(txt, 80)
				}
			}
		case "SYSTEM":
			// Keep system messages. CHECKPOINT records with empty content are
			// noise and can be skipped, but non-empty ones carry useful context
			// (e.g. handoff provenance headers).
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

// deriveIDFromPath extracts a conversation UUID from a brain path.
func deriveIDFromPath(path string) string {
	// Try to extract from /brain/<uuid>/... pattern
	if strings.Contains(path, "/brain/") {
		parts := strings.Split(filepath.ToSlash(path), "/brain/")
		if len(parts) > 1 {
			rest := parts[1]
			if idx := strings.Index(rest, "/"); idx > 0 {
				return rest[:idx]
			}
			return rest
		}
	}
	// Fallback: try parent directory name
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if base == "transcript" || base == "transcript_full" {
		// path like .../logs/transcript.jsonl → go up to conversation dir
		return filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
	}
	if base == "handoff_context" {
		// path like .../<uuid>/handoff_context.md
		return filepath.Base(filepath.Dir(path))
	}
	return filepath.Base(filepath.Dir(path))
}

func (a *Adapter) DefaultTarget(conv *models.Conversation) (string, error) {
	id := adapters.NewUUID()
	root := brainRoot()
	return filepath.Join(root, id, "handoff_context.md"), nil
}

func (a *Adapter) Inject(conv *models.Conversation, targetPath string) (string, error) {
	var err error
	if strings.TrimSpace(targetPath) == "" {
		targetPath, err = a.DefaultTarget(conv)
		if err != nil {
			return "", err
		}
	}

	// If target is a directory or brain ID, construct the handoff path
	if info, statErr := os.Stat(targetPath); statErr == nil && info.IsDir() {
		targetPath = filepath.Join(targetPath, "handoff_context.md")
	} else if !strings.HasSuffix(targetPath, ".md") && !strings.HasSuffix(targetPath, ".jsonl") {
		// Assume it's a brain ID
		if !strings.Contains(targetPath, "/") {
			targetPath = filepath.Join(brainRoot(), targetPath, "handoff_context.md")
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}

	// Write the markdown handoff file
	mdPath := targetPath
	if strings.HasSuffix(targetPath, ".jsonl") {
		// Legacy path — write transcript.jsonl AND handoff markdown next to it
		mdPath = filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(targetPath))), "handoff_context.md")
		if err := writeTranscriptJSONL(conv, targetPath); err != nil {
			return "", err
		}
	}

	if err := os.MkdirAll(filepath.Dir(mdPath), 0o755); err != nil {
		return "", err
	}
	if err := writeHandoffMarkdown(conv, mdPath); err != nil {
		return "", err
	}

	return mdPath, nil
}

// writeHandoffMarkdown formats a conversation as a readable markdown document
// that can be pasted into an Antigravity chat or referenced via @mention.
func writeHandoffMarkdown(conv *models.Conversation, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	w := bufio.NewWriter(f)

	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = "Handoff Context"
	}

	fmt.Fprintf(w, "# %s\n\n", title)
	fmt.Fprintf(w, "> Handed off via Vibeporter from **%s** (id: `%s`)\n\n", conv.AgentSource, conv.ID)
	fmt.Fprintln(w, "---")
	fmt.Fprintln(w)

	for _, msg := range conv.Messages {
		switch msg.Role {
		case models.RoleUser:
			fmt.Fprintln(w, "## 👤 User")
			fmt.Fprintln(w)
			fmt.Fprintln(w, msg.StringContent())
			fmt.Fprintln(w)
		case models.RoleAssistant:
			fmt.Fprintln(w, "## 🤖 Assistant")
			fmt.Fprintln(w)
			for _, p := range msg.EffectiveParts() {
				switch p.Kind {
				case models.PartThinking:
					fmt.Fprintln(w, "<details>")
					fmt.Fprintln(w, "<summary>Thinking</summary>")
					fmt.Fprintln(w)
					fmt.Fprintln(w, p.Text)
					fmt.Fprintln(w)
					fmt.Fprintln(w, "</details>")
					fmt.Fprintln(w)
				case models.PartText:
					fmt.Fprintln(w, p.Text)
					fmt.Fprintln(w)
				case models.PartToolCall:
					fmt.Fprintf(w, "**Tool call**: `%s`", p.Name)
					if strings.TrimSpace(p.ArgsJSON) != "" && p.ArgsJSON != "{}" {
						fmt.Fprintf(w, "\n```json\n%s\n```", p.ArgsJSON)
					}
					fmt.Fprintln(w)
					fmt.Fprintln(w)
				case models.PartToolResult:
					fmt.Fprintln(w, "<details>")
					fmt.Fprintln(w, "<summary>Tool result</summary>")
					fmt.Fprintln(w)
					if strings.TrimSpace(p.Text) != "" {
						fmt.Fprintln(w, "```")
						fmt.Fprintln(w, p.Text)
						fmt.Fprintln(w, "```")
					}
					fmt.Fprintln(w)
					fmt.Fprintln(w, "</details>")
					fmt.Fprintln(w)
				}
			}
		case models.RoleSystem:
			fmt.Fprintln(w, "## ⚙️ System")
			fmt.Fprintln(w)
			fmt.Fprintln(w, msg.StringContent())
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, "---")
		fmt.Fprintln(w)
	}

	return w.Flush()
}

// writeTranscriptJSONL writes the conversation as transcript.jsonl for backward
// compatibility with vibeporter list/stats/search commands.
func writeTranscriptJSONL(conv *models.Conversation, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
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
			rec["type"] = "GENERIC"
			rec["content"] = msg.StringContent()
		case models.RoleAssistant:
			rec["source"] = "MODEL"
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
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
	}
	return w.Flush()
}
