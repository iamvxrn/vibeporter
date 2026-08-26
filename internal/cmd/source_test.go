package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

type fakeExtractor struct {
	chats []adapters.ChatInfo
}

func (f fakeExtractor) Extract(string) (*models.Conversation, error) {
	return nil, nil
}

func (f fakeExtractor) ListConversations() ([]adapters.ChatInfo, error) {
	return f.chats, nil
}

func TestResolveSourceExactID(t *testing.T) {
	ex := fakeExtractor{chats: []adapters.ChatInfo{
		{ID: "aaa-111", Path: "/tmp/a.jsonl"},
		{ID: "bbb-222", Path: "/tmp/b.jsonl"},
	}}
	got, err := resolveSource(ex, "bbb-222")
	if err != nil || got != "/tmp/b.jsonl" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestResolveSourceUniquePrefix(t *testing.T) {
	ex := fakeExtractor{chats: []adapters.ChatInfo{
		{ID: "aaa-111", Path: "/tmp/a.jsonl"},
		{ID: "bbb-222", Path: "/tmp/b.jsonl"},
	}}
	got, err := resolveSource(ex, "bbb")
	if err != nil || got != "/tmp/b.jsonl" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestResolveSourceAmbiguousPrefix(t *testing.T) {
	ex := fakeExtractor{chats: []adapters.ChatInfo{
		{ID: "aaa-111", Path: "/tmp/a.jsonl"},
		{ID: "aaa-222", Path: "/tmp/b.jsonl"},
	}}
	if _, err := resolveSource(ex, "aaa"); err == nil {
		t.Fatal("expected ambiguity error")
	}
}

func TestResolveSourceExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chat.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ex := fakeExtractor{chats: []adapters.ChatInfo{
		{ID: "other", Path: "/nope"},
	}}
	got, err := resolveSource(ex, path)
	if err != nil || got != path {
		t.Fatalf("got %q err %v", got, err)
	}
}
