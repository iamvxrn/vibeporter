package opencode

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	dbPath := getDBPath()
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("opencode db not found: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, title FROM session ORDER BY time_created DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []adapters.ChatInfo
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err == nil {
			chats = append(chats, adapters.ChatInfo{
				ID:    id + " (" + title + ")",
				Path:  id, // We use the ID as the path representation for SQLite
				Agent: "opencode",
			})
		}
	}
	return chats, nil
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
