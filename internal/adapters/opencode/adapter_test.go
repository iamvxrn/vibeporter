package opencode

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vibeporter/internal/models"

	_ "modernc.org/sqlite"
)

func TestNewAdapterAndDefaultTarget(t *testing.T) {
	a := NewAdapter()
	if a == nil {
		t.Fatal("nil adapter")
	}
	got, err := a.DefaultTarget(&models.Conversation{})
	if err != nil {
		t.Fatal(err)
	}
	if got == "" || !strings.Contains(got, "opencode.db") {
		t.Fatalf("DefaultTarget: %q", got)
	}
}

func TestListSessionsReadsTitleAndDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE session (
			id TEXT,
			title TEXT,
			directory TEXT,
			time_created INTEGER,
			time_updated INTEGER
		);
		INSERT INTO session (id, title, directory, time_created, time_updated)
		VALUES ('ses_abc', 'Нужен ли README.md', '/home/someone/extra/git/cymose-dev', 1000, 1787762823782);
		INSERT INTO session (id, title, directory, time_created, time_updated)
		VALUES ('ses_empty', '  ', '/tmp', 0, 0);
		INSERT INTO session (id, title, directory, time_created, time_updated)
		VALUES ('ses_sec', 'seconds', '', 1700000000, NULL);
	`)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	chats, err := listSessions(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 3 {
		t.Fatalf("len=%d", len(chats))
	}
	byID := map[string]int{}
	for i, c := range chats {
		byID[c.ID] = i
	}
	c := chats[byID["ses_abc"]]
	if c.ID != "ses_abc" || c.Path != "ses_abc" {
		t.Fatalf("id/path: %+v", c)
	}
	if c.Title != "Нужен ли README.md" {
		t.Fatalf("title: %q", c.Title)
	}
	if c.UpdatedAt.UnixMilli() != 1787762823782 {
		t.Fatalf("updated: %s (%d)", c.UpdatedAt, c.UpdatedAt.UnixMilli())
	}
	if !c.UpdatedAt.Equal(time.UnixMilli(1787762823782)) {
		t.Fatalf("updated mismatch")
	}
	if chats[byID["ses_empty"]].Title != "Untitled" {
		t.Fatalf("empty title: %q", chats[byID["ses_empty"]].Title)
	}
	sec := chats[byID["ses_sec"]].UpdatedAt
	if !sec.Equal(time.Unix(1700000000, 0)) {
		t.Fatalf("seconds updated: %v", sec)
	}
}

func TestListSessionsMissingDB(t *testing.T) {
	if _, err := listSessions(filepath.Join(t.TempDir(), "missing.db")); err == nil {
		t.Fatal("expected missing db error")
	}
}

func TestListSessionsLegacySchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE session (id TEXT, title TEXT, time_created INTEGER);
		INSERT INTO session (id, title, time_created) VALUES ('ses_old', '', 1);
	`)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}
	chats, err := listSessions(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0].ID != "ses_old" || chats[0].Title != "Untitled" {
		t.Fatalf("legacy: %+v", chats)
	}
}

func TestUnixMillis(t *testing.T) {
	if !unixMillis(0).IsZero() || !unixMillis(-1).IsZero() {
		t.Fatal("non-positive should be zero")
	}
	if !unixMillis(100).Equal(time.Unix(100, 0)) {
		t.Fatal("seconds")
	}
	if !unixMillis(1_700_000_000_000).Equal(time.UnixMilli(1_700_000_000_000)) {
		t.Fatal("millis")
	}
}

func TestPartText(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`not-json`, ""},
		{`{"type":"text","text":"hello"}`, "hello\n"},
		{`{"type":"text","text":"  "}`, ""},
		{`{"type":"reasoning","text":"secret"}`, ""},
		{`{"type":"tool","tool":"bash"}`, "[Tool Use: bash]\n"},
		{`{"type":"tool"}`, "[Tool Use: tool]\n"},
		{`{"type":"image"}`, ""},
	}
	for _, tc := range cases {
		if got := partText(tc.raw); got != tc.want {
			t.Errorf("partText(%s)=%q want %q", tc.raw, got, tc.want)
		}
	}
}

func TestExtractSession(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	db := mustOpen(t, dbPath)
	_, err := db.Exec(`
		CREATE TABLE session (id TEXT, title TEXT, directory TEXT);
		CREATE TABLE message (id TEXT, session_id TEXT, data TEXT, time_created INTEGER);
		CREATE TABLE part (id TEXT, message_id TEXT, session_id TEXT, time_created INTEGER, data TEXT);
		INSERT INTO session VALUES ('ses_1', ' Demo ', '/work');
		INSERT INTO message VALUES ('msg_u', 'ses_1', '{"role":"user"}', 1700000000000);
		INSERT INTO message VALUES ('msg_a', 'ses_1', '{"role":"assistant"}', 1700000001000);
		INSERT INTO message VALUES ('msg_empty', 'ses_1', '{"role":"assistant"}', 1700000002000);
		INSERT INTO part VALUES ('p1', 'msg_u', 'ses_1', 1, '{"type":"text","text":"hi"}');
		INSERT INTO part VALUES ('p2', 'msg_a', 'ses_1', 1, '{"type":"reasoning","text":"think"}');
		INSERT INTO part VALUES ('p3', 'msg_a', 'ses_1', 2, '{"type":"text","text":"ok"}');
		INSERT INTO part VALUES ('p4', 'msg_a', 'ses_1', 3, '{"type":"tool","tool":"bash"}');
		INSERT INTO part VALUES ('p5', 'msg_empty', 'ses_1', 1, '{"type":"reasoning","text":"skip"}');
	`)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}

	conv, err := extractSession(dbPath, "ses_1")
	if err != nil {
		t.Fatal(err)
	}
	if conv.Title != "Demo" || conv.AgentSource != "opencode" || conv.Metadata["cwd"] != "/work" {
		t.Fatalf("meta: %+v", conv)
	}
	if len(conv.Messages) != 3 {
		t.Fatalf("messages=%d %+v", len(conv.Messages), conv.Messages)
	}
	if conv.Messages[0].Role != models.RoleUser || conv.Messages[0].Content != "hi" {
		t.Fatalf("user: %+v", conv.Messages[0])
	}
	if conv.Messages[1].Role != models.RoleAssistant {
		t.Fatalf("assistant role: %+v", conv.Messages[1])
	}
	if !strings.Contains(conv.Messages[1].Content, "ok") || !strings.Contains(conv.Messages[1].Content, "[Tool Use: bash]") {
		t.Fatalf("assistant content: %q", conv.Messages[1].Content)
	}
	kinds := []models.PartKind{}
	for _, p := range conv.Messages[1].Parts {
		kinds = append(kinds, p.Kind)
	}
	if len(kinds) != 3 || kinds[0] != models.PartThinking || kinds[1] != models.PartText || kinds[2] != models.PartToolCall {
		t.Fatalf("asst parts: %+v", conv.Messages[1].Parts)
	}
	if conv.Messages[2].Parts[0].Kind != models.PartThinking || conv.Messages[2].Parts[0].Text != "skip" {
		t.Fatalf("thinking-only: %+v", conv.Messages[2])
	}
}

func TestInjectExtractRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	createInjectSchema(t, dbPath)
	ts := time.UnixMilli(1_700_000_000_000)
	in := &models.Conversation{
		Title: "imported",
		Messages: []models.Message{
			{Role: models.RoleUser, Content: "hello", Timestamp: &ts},
			{Role: models.RoleAssistant, Content: "world"},
		},
		Metadata: map[string]interface{}{"cwd": "/proj"},
	}
	id, err := injectSession(dbPath, in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(id, "ses_") {
		t.Fatalf("id: %q", id)
	}
	chats, err := listSessions(dbPath)
	if err != nil || len(chats) != 1 || chats[0].ID != id || chats[0].Title != "imported" {
		t.Fatalf("list: %+v err=%v", chats, err)
	}
	out, err := extractSession(dbPath, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 2 || out.Messages[0].Content != "hello" || out.Messages[1].Content != "world" {
		t.Fatalf("round-trip: %+v", out.Messages)
	}
	if out.Metadata["cwd"] != "/proj" {
		t.Fatalf("cwd: %+v", out.Metadata)
	}
}

func TestInjectExtractPreservesThinkingAndTools(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	createInjectSchema(t, dbPath)
	in := &models.Conversation{
		Title: "parts",
		Messages: []models.Message{
			models.NewMessage(models.RoleAssistant, []models.Part{
				models.ThinkingPart("plan"),
				models.TextPart("ok"),
				models.ToolCallPart("", "bash", `{"cmd":"ls"}`),
			}),
		},
	}
	id, err := injectSession(dbPath, in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := extractSession(dbPath, id)
	if err != nil {
		t.Fatal(err)
	}
	// Tool calls are now preserved even without output (fix for cursor→opencode fidelity)
	if len(out.Messages) != 1 || len(out.Messages[0].Parts) != 3 {
		t.Fatalf("round-trip parts: %+v", out.Messages)
	}
	if out.Messages[0].Parts[0].Kind != models.PartThinking || out.Messages[0].Parts[1].Text != "ok" || out.Messages[0].Parts[2].Kind != models.PartToolCall {
		t.Fatalf("parts: %+v", out.Messages[0].Parts)
	}
}

func TestInjectWritesOpenCodeSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	createInjectSchema(t, dbPath)
	in := &models.Conversation{
		Title: "schema",
		Messages: []models.Message{
			models.NewMessage(models.RoleUser, []models.Part{models.TextPart("q")}),
			models.NewMessage(models.RoleAssistant, []models.Part{
				models.ThinkingPart("plan"),
				models.TextPart("ok"),
				models.ToolCallPart("c1", "bash", `{"cmd":"ls"}`),
			}),
			models.NewMessage(models.RoleUser, []models.Part{
				models.ToolResultPart("c1", "a.txt", false),
			}),
		},
		Metadata: map[string]interface{}{"cwd": "/proj"},
	}
	id, err := injectSession(dbPath, in)
	if err != nil {
		t.Fatal(err)
	}
	db := mustOpen(t, dbPath)
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message WHERE session_id = ?`, id).Scan(&n); err != nil || n != 2 {
		t.Fatalf("messages=%d err=%v", n, err)
	}
	var asstData string
	if err := db.QueryRow(`SELECT data FROM message WHERE session_id = ? AND json_extract(data,'$.role')='assistant'`, id).Scan(&asstData); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(asstData, `"parentID":"msg_`) {
		t.Fatalf("missing parentID: %s", asstData)
	}
	if !strings.Contains(asstData, `"agent":"build"`) {
		t.Fatalf("missing agent: %s", asstData)
	}
	var toolData string
	if err := db.QueryRow(`SELECT data FROM part WHERE session_id = ? AND json_extract(data,'$.type')='tool'`, id).Scan(&toolData); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(toolData, `"callID"`) || !strings.Contains(toolData, `"status":"completed"`) {
		t.Fatalf("tool part: %s", toolData)
	}
	if !strings.Contains(toolData, "a.txt") {
		t.Fatalf("tool output not merged: %s", toolData)
	}
}

func TestInjectExtractSystem(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	createInjectSchema(t, dbPath)
	in := &models.Conversation{
		Title: "sys",
		Messages: []models.Message{
			models.NewMessage(models.RoleSystem, []models.Part{models.TextPart("be terse")}),
			models.NewMessage(models.RoleUser, []models.Part{models.TextPart("hi")}),
		},
	}
	id, err := injectSession(dbPath, in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := extractSession(dbPath, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("n=%d", len(out.Messages))
	}
	if out.Messages[0].Role != models.RoleUser || out.Messages[0].Content != "be terse" {
		t.Fatalf("system mapped to user: %+v", out.Messages[0])
	}
}

func TestInjectDefaultTitleAndCwd(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	createInjectSchema(t, dbPath)
	id, err := injectSession(dbPath, &models.Conversation{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := extractSession(dbPath, id)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Imported session" {
		t.Fatalf("title: %q", out.Title)
	}
}

func TestAdapterHomeDB(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	dbPath := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	createInjectSchema(t, dbPath)
	if _, err := injectSession(dbPath, &models.Conversation{Title: "home"}); err != nil {
		t.Fatal(err)
	}
	a := NewAdapter()
	chats, err := a.ListConversations()
	if err != nil || len(chats) != 1 {
		t.Fatalf("list: %+v err=%v", chats, err)
	}
	conv, err := a.Extract(chats[0].ID)
	if err != nil || conv.Title != "home" {
		t.Fatalf("extract: %+v err=%v", conv, err)
	}
	id, err := a.Inject(&models.Conversation{Title: "via-adapter"}, "")
	if err != nil || !strings.HasPrefix(id, "ses_") {
		t.Fatalf("inject: %q %v", id, err)
	}
}

func TestAdapterListMissingHomeDB(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	if _, err := NewAdapter().ListConversations(); err == nil {
		t.Fatal("expected missing db")
	}
}

func TestGetDBPathPrefersExistingXDG(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	want := filepath.Join(xdg, "opencode", "opencode.db")
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := getDBPath(); got != want {
		t.Fatalf("getDBPath = %q want %q", got, want)
	}
}

func TestGetDBPathDefaultWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	got := getDBPath()
	if !strings.HasSuffix(got, "opencode.db") {
		t.Fatalf("getDBPath = %q", got)
	}
}

func mustOpen(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func createInjectSchema(t *testing.T, dbPath string) {
	t.Helper()
	db := mustOpen(t, dbPath)
	_, err := db.Exec(`
		CREATE TABLE session (
			id TEXT, project_id TEXT, slug TEXT, directory TEXT, path TEXT, title TEXT, version TEXT,
			cost REAL, tokens_input INTEGER, tokens_output INTEGER, tokens_reasoning INTEGER,
			tokens_cache_read INTEGER, tokens_cache_write INTEGER,
			time_created INTEGER, time_updated INTEGER
		);
		CREATE TABLE message (
			id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT
		);
		CREATE TABLE part (
			id TEXT, message_id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT
		);
	`)
	_ = db.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestInjectCreatesSchemaAndHonorsTarget(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "custom", "opencode.db")
	a := NewAdapter()
	id, err := a.Inject(&models.Conversation{Title: "fresh-db"}, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	chats, err := listSessions(dbPath)
	if err != nil || len(chats) != 1 || chats[0].ID != id {
		t.Fatalf("list custom db: %+v err=%v", chats, err)
	}
}
