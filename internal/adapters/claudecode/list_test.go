package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSummarizeFileUsesAITitleAndCwd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "203f4afc-2fd1-40e9-be7b-fc26d8fc0759.jsonl")
	body := strings.Join([]string{
		`{"type":"queue-operation","timestamp":"2026-08-10T10:51:56.641Z"}`,
		`{"type":"user","isMeta":true,"cwd":"/should/not/win","message":{"content":[{"type":"text","text":"meta"}]}}`,
		`{"type":"user","cwd":"/home/someone/extra/git/pkgline","timestamp":"2026-08-10T10:52:00Z","message":{"content":[{"type":"text","text":"привет мир"}]}}`,
		`{"type":"ai-title","aiTitle":"Минимальный прототип","sessionId":"203f4afc-2fd1-40e9-be7b-fc26d8fc0759"}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got := summarizeFile(path, time.Time{})
	if got.Title != "Минимальный прототип" {
		t.Fatalf("title: %q", got.Title)
	}
	if got.ID != "203f4afc-2fd1-40e9-be7b-fc26d8fc0759" {
		t.Fatalf("id: %q", got.ID)
	}
	if !strings.Contains(got.Project, "pkgline") && got.Project != "/home/someone/extra/git/pkgline" {
		t.Fatalf("project: %q", got.Project)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("expected timestamp")
	}
}

func TestSummarizeFileSkipsSlashCommandUser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sess.jsonl")
	body := strings.Join([]string{
		`{"type":"user","message":{"content":[{"type":"text","text":"<command-name>/model</command-name>"}]}}`,
		`{"type":"user","message":{"content":[{"type":"text","text":"переключи модель"}]}}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := summarizeFile(path, time.Time{})
	if got.Title != "переключи модель" {
		t.Fatalf("title: %q", got.Title)
	}
}

func TestSummarizeFileFallsBackToFirstUser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sess.jsonl")
	body := `{"type":"user","message":{"content":[{"type":"text","text":"новая сессия"}]}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := summarizeFile(path, time.Time{})
	if got.Title != "новая сессия" {
		t.Fatalf("title: %q", got.Title)
	}
}
