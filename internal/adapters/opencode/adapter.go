package opencode

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vibeporter/internal/adapters"
	"vibeporter/internal/models"

	_ "modernc.org/sqlite"
)

type Adapter struct{}

func NewAdapter() *Adapter {
	return &Adapter{}
}

func getDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}

func (a *Adapter) ListConversations() ([]adapters.ChatInfo, error) {
	return listSessions(getDBPath())
}

func listSessions(dbPath string) ([]adapters.ChatInfo, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("opencode db not found: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	chats, err := querySessions(db, `SELECT id, title, directory, time_updated, time_created FROM session ORDER BY COALESCE(time_updated, time_created) DESC`)
	if err != nil {
		chats, err = querySessionsLegacy(db)
	}
	return chats, err
}

func querySessions(db *sql.DB, q string) ([]adapters.ChatInfo, error) {
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []adapters.ChatInfo
	for rows.Next() {
		var id, title, directory string
		var updated, created sql.NullInt64
		if err := rows.Scan(&id, &title, &directory, &updated, &created); err != nil {
			continue
		}
		info := adapters.ChatInfo{
			ID:      id,
			Path:    id,
			Agent:   "opencode",
			Title:   strings.TrimSpace(title),
			Project: adapters.ShortPath(directory),
		}
		if info.Title == "" {
			info.Title = "Untitled"
		}
		if updated.Valid {
			info.UpdatedAt = unixMillis(updated.Int64)
		} else if created.Valid {
			info.UpdatedAt = unixMillis(created.Int64)
		}
		chats = append(chats, info)
	}
	return chats, rows.Err()
}

func querySessionsLegacy(db *sql.DB) ([]adapters.ChatInfo, error) {
	rows, err := db.Query(`SELECT id, title FROM session ORDER BY time_created DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []adapters.ChatInfo
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			continue
		}
		if strings.TrimSpace(title) == "" {
			title = "Untitled"
		}
		chats = append(chats, adapters.ChatInfo{
			ID:    id,
			Path:  id,
			Agent: "opencode",
			Title: strings.TrimSpace(title),
		})
	}
	return chats, rows.Err()
}

func unixMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	if ms < 1e12 {
		return time.Unix(ms, 0)
	}
	return time.UnixMilli(ms)
}

func (a *Adapter) Extract(sessionID string) (*models.Conversation, error) {
	dbPath := getDBPath()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	conv := &models.Conversation{
		ID:          sessionID,
		AgentSource: "opencode",
		Messages:    []models.Message{},
	}

	query := `
		SELECT m.data, p.data 
		FROM message m 
		JOIN part p ON m.id = p.message_id 
		WHERE m.session_id = ? 
		ORDER BY m.time_created ASC, p.time_created ASC
	`
	rows, err := db.Query(query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var msgDataStr, partDataStr string
		if err := rows.Scan(&msgDataStr, &partDataStr); err != nil {
			continue
		}

		var msgData map[string]interface{}
		var partData map[string]interface{}

		json.Unmarshal([]byte(msgDataStr), &msgData)
		json.Unmarshal([]byte(partDataStr), &partData)

		roleStr, _ := msgData["role"].(string)

		role := models.RoleSystem
		if roleStr == "user" {
			role = models.RoleUser
		} else if roleStr == "assistant" {
			role = models.RoleAssistant
		}

		content := ""
		if txt, ok := partData["text"].(string); ok {
			content = txt
		}

		if content != "" {
			conv.Messages = append(conv.Messages, models.Message{
				Role:    role,
				Content: content,
			})
		}
	}

	return conv, nil
}
