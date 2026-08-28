package opencode

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	for _, p := range opencodeDBCandidates() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	cands := opencodeDBCandidates()
	if len(cands) == 0 {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	}
	return cands[0]
}

func opencodeDBCandidates() []string {
	home, _ := os.UserHomeDir()
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		add(filepath.Join(xdg, "opencode", "opencode.db"))
	}
	switch runtime.GOOS {
	case "darwin":
		add(filepath.Join(home, "Library", "Application Support", "opencode", "opencode.db"))
	case "windows":
		if app := os.Getenv("APPDATA"); app != "" {
			add(filepath.Join(app, "opencode", "opencode.db"))
		} else {
			add(filepath.Join(home, "AppData", "Roaming", "opencode", "opencode.db"))
		}
	}
	add(filepath.Join(home, ".local", "share", "opencode", "opencode.db"))
	return out
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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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
	return extractSession(getDBPath(), sessionID)
}

func extractSession(dbPath, sessionID string) (*models.Conversation, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	conv := &models.Conversation{
		ID:          sessionID,
		AgentSource: "opencode",
		Messages:    []models.Message{},
		Metadata:    map[string]interface{}{},
	}

	var title, directory string
	_ = db.QueryRow(`SELECT title, directory FROM session WHERE id = ?`, sessionID).Scan(&title, &directory)
	conv.Title = strings.TrimSpace(title)
	if directory != "" {
		conv.Metadata["cwd"] = directory
	}

	msgRows, err := db.Query(`SELECT id, data, time_created FROM message WHERE session_id = ? ORDER BY time_created ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = msgRows.Close() }()

	type msgRow struct {
		id      string
		data    string
		created int64
	}
	var msgs []msgRow
	for msgRows.Next() {
		var m msgRow
		if err := msgRows.Scan(&m.id, &m.data, &m.created); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}

	for _, m := range msgs {
		var msgData map[string]interface{}
		_ = json.Unmarshal([]byte(m.data), &msgData)
		roleStr, _ := msgData["role"].(string)
		role := models.RoleAssistant
		if roleStr == "user" {
			role = models.RoleUser
		}

		partRows, err := db.Query(`SELECT data FROM part WHERE message_id = ? ORDER BY time_created ASC`, m.id)
		if err != nil {
			continue
		}
		var b strings.Builder
		for partRows.Next() {
			var partDataStr string
			if err := partRows.Scan(&partDataStr); err != nil {
				continue
			}
			b.WriteString(partText(partDataStr))
		}
		_ = partRows.Close()
		text := strings.TrimSpace(b.String())
		if text == "" {
			continue
		}
		msg := models.Message{Role: role, Content: text, Timestamp: adapters.UnixMillisPtr(m.created)}
		conv.Messages = append(conv.Messages, msg)
	}

	return conv, nil
}

func partText(raw string) string {
	var part map[string]interface{}
	if json.Unmarshal([]byte(raw), &part) != nil {
		return ""
	}
	switch part["type"] {
	case "text", "reasoning":
		if txt, ok := part["text"].(string); ok && strings.TrimSpace(txt) != "" {
			if part["type"] == "reasoning" {
				return ""
			}
			return txt + "\n"
		}
	case "tool":
		name, _ := part["tool"].(string)
		if name == "" {
			name = "tool"
		}
		return fmt.Sprintf("[Tool Use: %s]\n", name)
	}
	return ""
}

func (a *Adapter) DefaultTarget(*models.Conversation) (string, error) {
	return getDBPath(), nil
}

func (a *Adapter) Inject(conv *models.Conversation, target string) (string, error) {
	dbPath := strings.TrimSpace(target)
	if dbPath == "" {
		dbPath = getDBPath()
	}
	return injectSession(dbPath, conv)
}

func ensureOpenCodeSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS session (
			id TEXT, project_id TEXT, slug TEXT, directory TEXT, path TEXT, title TEXT, version TEXT,
			cost REAL, tokens_input INTEGER, tokens_output INTEGER, tokens_reasoning INTEGER,
			tokens_cache_read INTEGER, tokens_cache_write INTEGER,
			time_created INTEGER, time_updated INTEGER
		);
		CREATE TABLE IF NOT EXISTS message (
			id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT
		);
		CREATE TABLE IF NOT EXISTS part (
			id TEXT, message_id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT
		);
	`)
	return err
}

func injectSession(dbPath string, conv *models.Conversation) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return "", err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()
	if err := ensureOpenCodeSchema(db); err != nil {
		return "", err
	}

	now := time.Now().UnixMilli()
	sessionID := adapters.NewPrefixedID("ses_")
	cwd := adapters.CwdFromMeta(conv)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if cwd == "" {
		cwd, _ = os.UserHomeDir()
	}
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = "Imported session"
	}
	slug := adapters.NewPrefixedID("imp-")
	rel := strings.TrimPrefix(filepath.ToSlash(cwd), "/")

	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT INTO session (
			id, project_id, slug, directory, path, title, version,
			cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write,
			time_created, time_updated
		) VALUES (?, 'global', ?, ?, ?, ?, '1.0.0', 0, 0, 0, 0, 0, 0, ?, ?)`,
		sessionID, slug, cwd, rel, title, now, now)
	if err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}

	for _, msg := range conv.Messages {
		ms := now
		if msg.Timestamp != nil {
			ms = msg.Timestamp.UnixMilli()
		}
		msgID := adapters.NewPrefixedID("msg_")
		role := "assistant"
		if msg.Role == models.RoleUser {
			role = "user"
		}
		msgData, _ := json.Marshal(map[string]interface{}{
			"role": role,
			"time": map[string]int64{"created": ms},
		})
		if _, err := tx.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?)`,
			msgID, sessionID, ms, ms, string(msgData)); err != nil {
			return "", fmt.Errorf("insert message: %w", err)
		}
		partID := adapters.NewPrefixedID("prt_")
		partData, _ := json.Marshal(map[string]interface{}{"type": "text", "text": msg.Content})
		if _, err := tx.Exec(`INSERT INTO part (id, message_id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?, ?)`,
			partID, msgID, sessionID, ms, ms, string(partData)); err != nil {
			return "", fmt.Errorf("insert part: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return sessionID, nil
}
