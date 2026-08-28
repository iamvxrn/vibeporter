package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

func TestRootHelp(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})
	if err := rootCmd.Help(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"list", "migrate", "port-config"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q:\n%s", want, out)
		}
	}
}

func TestListUnknownAgent(t *testing.T) {
	err := listCmd.RunE(listCmd, []string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("err=%v", err)
	}
}

func TestMigrateUnknownAgents(t *testing.T) {
	t.Cleanup(resetMigrateFlags)
	fromAgent, toAgent, sourcePath, targetPath = "nope", "gemini", "x", ""
	if err := migrateCmd.RunE(migrateCmd, nil); err == nil || !strings.Contains(err.Error(), "source agent") {
		t.Fatalf("from: %v", err)
	}
	fromAgent, toAgent = "gemini", "nope"
	if err := migrateCmd.RunE(migrateCmd, nil); err == nil || !strings.Contains(err.Error(), "target agent") {
		t.Fatalf("to: %v", err)
	}
}

func TestRegistryAliases(t *testing.T) {
	for _, name := range []string{"gemini", "claudecode", "opencode", "kimicode", "kimi", "dsh", "dhs"} {
		if _, ok := extractors[name]; !ok {
			t.Errorf("missing extractor %s", name)
		}
		if _, ok := injectors[name]; !ok {
			t.Errorf("missing injector %s", name)
		}
	}
}

func TestPrintListHuman(t *testing.T) {
	empty := captureStdout(t, func() { printListHuman("gemini", nil, false) })
	if !strings.Contains(empty, "No chats found") {
		t.Fatalf("empty: %q", empty)
	}
	one := captureStdout(t, func() {
		printListHuman("gemini", []adapters.ChatInfo{{
			ID: "abc", Title: "", Project: "p", Path: "/tmp/a", Agent: "gemini",
			UpdatedAt: time.Unix(1_700_000_000, 0).UTC(),
		}}, true)
	})
	if !strings.Contains(one, "1 chat") || !strings.Contains(one, "Untitled") || !strings.Contains(one, "PATH") {
		t.Fatalf("one: %q", one)
	}
	many := captureStdout(t, func() {
		printListHuman("gemini", []adapters.ChatInfo{
			{ID: "a", Title: "one\tline", Path: "a"},
			{ID: "b", Title: "two", Path: "b"},
		}, false)
	})
	if !strings.Contains(many, "2 chats") || !strings.Contains(many, "one line") {
		t.Fatalf("many: %q", many)
	}
}

func TestWriteListJSON(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	out := captureStdout(t, func() {
		if err := writeListJSON([]adapters.ChatInfo{{
			ID: "id1", Title: "t", Project: "p", Path: "/x", Agent: "gemini", UpdatedAt: ts,
		}}); err != nil {
			t.Fatal(err)
		}
	})
	var rows []map[string]string
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(rows) != 1 || rows[0]["id"] != "id1" || rows[0]["updated"] == "" {
		t.Fatalf("rows: %+v", rows)
	}
}

func TestPortFileCopyAndSkip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() { portFile(dir, "CLAUDE.md", "GEMINI.md") })
	if !strings.Contains(out, "Successfully ported") {
		t.Fatalf("copy: %q", out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "GEMINI.md"))
	if err != nil || string(raw) != "hello" {
		t.Fatalf("dest: %q %v", raw, err)
	}
	skip := captureStdout(t, func() { portFile(dir, "CLAUDE.md", "GEMINI.md") })
	if !strings.Contains(skip, "already exists") {
		t.Fatalf("skip: %q", skip)
	}
	missing := captureStdout(t, func() { portFile(dir, "NOPE.md", "X.md") })
	if missing != "" {
		t.Fatalf("missing should be silent: %q", missing)
	}
}

func TestPortConfigMappings(t *testing.T) {
	cases := []struct {
		from, to, src, dst string
	}{
		{"claudecode", "gemini", "CLAUDE.md", "GEMINI.md"},
		{"cursor", "gemini", ".cursorrules", "GEMINI.md"},
		{"opencode", "gemini", "OPENCODE.md", "GEMINI.md"},
		{"kimi", "gemini", "AGENTS.md", "GEMINI.md"},
		{"gemini", "claudecode", "GEMINI.md", "CLAUDE.md"},
		{"gemini", "opencode", "GEMINI.md", "OPENCODE.md"},
		{"claudecode", "cursor", "CLAUDE.md", ".cursorrules"},
		{"opencode", "kimicode", "OPENCODE.md", "AGENTS.md"},
	}
	for _, tc := range cases {
		t.Run(tc.from+"-"+tc.to, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.src), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			fromAgent, toAgent, portDir = tc.from, tc.to, dir
			t.Cleanup(resetMigrateFlags)
			captureStdout(t, func() { portCmd.Run(portCmd, nil) })
			if _, err := os.Stat(filepath.Join(dir, tc.dst)); err != nil {
				t.Fatalf("expected %s: %v", tc.dst, err)
			}
		})
	}
	fromAgent, toAgent, portDir = "gemini", "unknown", t.TempDir()
	t.Cleanup(resetMigrateFlags)
	out := captureStdout(t, func() { portCmd.Run(portCmd, nil) })
	if !strings.Contains(out, "not supported yet") {
		t.Fatalf("unsupported: %q", out)
	}
}

func TestCopyFileMissing(t *testing.T) {
	if err := copyFile(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveSourceEmptyAndUnknown(t *testing.T) {
	ex := fakeExtractor{chats: []adapters.ChatInfo{{ID: "aaa-111", Path: "/tmp/a"}}}
	if _, err := resolveSource(ex, "  "); err == nil {
		t.Fatal("empty")
	}
	got, err := resolveSource(ex, "zzz")
	if err != nil || got != "zzz" {
		t.Fatalf("unknown id should pass through: %q %v", got, err)
	}
}

func TestResolveSourceListErrorPassesSpec(t *testing.T) {
	got, err := resolveSource(errExtractor{}, "ses_abc")
	if err != nil || got != "ses_abc" {
		t.Fatalf("got %q err %v", got, err)
	}
}

type errExtractor struct{}

func (errExtractor) Extract(string) (*models.Conversation, error) {
	return nil, fmt.Errorf("no extract")
}

func (errExtractor) ListConversations() ([]adapters.ChatInfo, error) {
	return nil, fmt.Errorf("no list")
}

func resetMigrateFlags() {
	fromAgent, toAgent, sourcePath, targetPath, portDir = "", "", "", "", ""
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String()
}
