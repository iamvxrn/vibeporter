package opencode

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

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
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	chats, err := listSessions(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Fatalf("len=%d", len(chats))
	}
	c := chats[0]
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
}
