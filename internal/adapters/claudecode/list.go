package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"vibeporter/internal/adapters"
)

func (a *Adapter) ListConversations() ([]adapters.ChatInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	var chats []adapters.ChatInfo

	err = filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") || strings.Contains(path, "subagents") {
			return nil
		}
		chats = append(chats, summarizeFile(path, info.ModTime()))
		return nil
	})

	return chats, err
}

func summarizeFile(path string, modTime time.Time) adapters.ChatInfo {
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	info := adapters.ChatInfo{
		ID:        id,
		Path:      path,
		Agent:     "claudecode",
		UpdatedAt: modTime,
	}

	var firstUser string
	_ = adapters.ForEachJSONLLimited(path, 256*1024, 128*1024, func(rec map[string]interface{}) {
		if meta, _ := rec["isMeta"].(bool); meta {
			return
		}
		if ts, _ := rec["timestamp"].(string); ts != "" {
			if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
				info.UpdatedAt = parsed
			} else if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				info.UpdatedAt = parsed
			}
		}
		if cwd, _ := rec["cwd"].(string); cwd != "" && info.Project == "" {
			info.Project = adapters.ShortPath(cwd)
		}
		if rec["type"] == "ai-title" {
			if title, _ := rec["aiTitle"].(string); strings.TrimSpace(title) != "" {
				info.Title = strings.TrimSpace(title)
			}
		}
		if rec["type"] == "user" && firstUser == "" {
			text := firstUserText(rec)
			if isSlashCommandDump(text) {
				return
			}
			firstUser = text
		}
	})

	if info.Title == "" {
		info.Title = adapters.Clip(firstUser, 80)
	}
	if info.Title == "" {
		info.Title = "Untitled"
	}
	return info
}

func isSlashCommandDump(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "<command-") || strings.HasPrefix(s, "<local-command")
}

func firstUserText(rec map[string]interface{}) string {
	messageObj, ok := rec["message"].(map[string]interface{})
	if !ok {
		return ""
	}
	switch content := messageObj["content"].(type) {
	case string:
		return strings.TrimSpace(content)
	case []interface{}:
		var b strings.Builder
		for _, part := range content {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}
			if partType, _ := partMap["type"].(string); partType == "text" {
				if text, ok := partMap["text"].(string); ok {
					b.WriteString(text)
					b.WriteString("\n")
				}
			}
		}
		return strings.TrimSpace(b.String())
	default:
		return ""
	}
}
