package cmd

import (
	"strings"
	"testing"

	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

func TestClipSnippet(t *testing.T) {
	s := clipSnippet("hello world fix database bug here", "database", 20)
	if !strings.Contains(strings.ToLower(s), "database") {
		t.Fatalf("snippet %q", s)
	}
	// truncation
	long := strings.Repeat("a ", 100) + "needle" + strings.Repeat(" b", 100)
	snip := clipSnippet(long, "needle", 30)
	if !strings.Contains(snip, "needle") || len([]rune(snip)) > 40 {
		t.Fatalf("clip %q len %d runes %d", snip, len(snip), len([]rune(snip)))
	}
	// no match returns head
	snip2 := clipSnippet("short text", "missing", 80)
	if snip2 != "short text" {
		t.Fatalf("no match %q", snip2)
	}
}

func TestMatchConversation(t *testing.T) {
	conv := &models.Conversation{
		Title: "Fix database bug",
		Messages: []models.Message{
			models.NewMessage(models.RoleUser, []models.Part{models.TextPart("hello")}),
			models.NewMessage(models.RoleAssistant, []models.Part{models.TextPart("fixed the auth panic")}),
			models.NewMessage(models.RoleAssistant, []models.Part{models.ThinkingPart("thinking about database")}),
		},
	}
	matches, snippet := matchConversation(conv, "database", "database")
	if matches == 0 || snippet == "" {
		t.Fatalf("want match %d %q", matches, snippet)
	}
	matches2, _ := matchConversation(conv, "missing", "missing")
	if matches2 != 0 {
		t.Fatalf("no match %d", matches2)
	}
	// tool call name matching
	conv2 := &models.Conversation{
		Messages: []models.Message{
			models.NewMessage(models.RoleAssistant, []models.Part{models.ToolCallPart("1", "write_file", `{"path":"fix.go"}`)}),
		},
	}
	m, _ := matchConversation(conv2, "write_file", "write_file")
	if m == 0 {
		t.Fatal("tool name should match")
	}
}

func TestTargetAgents(t *testing.T) {
	all := targetAgents("")
	if len(all) < 6 {
		t.Fatalf("all %v", all)
	}
	// sorted
	for i := 1; i < len(all); i++ {
		if all[i-1] > all[i] {
			t.Fatalf("not sorted %v", all)
		}
	}
	one := targetAgents("gemini")
	if len(one) != 1 || one[0] != "gemini" {
		t.Fatalf("one %v", one)
	}
	unknown := targetAgents("nope")
	if len(unknown) != 1 || unknown[0] != "nope" {
		t.Fatalf("unknown %v", unknown)
	}
}

// fakeSearchExtractor for runSearch integration
type fakeSearchExtractor struct {
	chats []adapters.ChatInfo
	convs map[string]*models.Conversation
}

func (f fakeSearchExtractor) ListConversations() ([]adapters.ChatInfo, error) {
	return f.chats, nil
}
func (f fakeSearchExtractor) Extract(path string) (*models.Conversation, error) {
	if c, ok := f.convs[path]; ok {
		return c, nil
	}
	return &models.Conversation{Title: "empty"}, nil
}

func TestRunSearch(t *testing.T) {
	// inject fake agent
	fake := fakeSearchExtractor{
		chats: []adapters.ChatInfo{
			{ID: "a1", Title: "fix database bug", Project: "proj", Path: "p1"},
			{ID: "b2", Title: "other", Project: "proj", Path: "p2"},
		},
		convs: map[string]*models.Conversation{
			"p1": {ID: "a1", Title: "fix database bug", Messages: []models.Message{models.NewMessage(models.RoleUser, []models.Part{models.TextPart("hello")})}},
			"p2": {ID: "b2", Title: "other", Messages: []models.Message{models.NewMessage(models.RoleUser, []models.Part{models.TextPart("fix database bug inside message")})}},
		},
	}
	orig, had := extractors["testagent"]
	extractors["testagent"] = fake
	defer func() {
		if had {
			extractors["testagent"] = orig
		} else {
			delete(extractors, "testagent")
		}
	}()

	hits, err := runSearch("database bug", []string{"testagent"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 hits got %d %+v", len(hits), hits)
	}
	if hits[0].Matches == 0 || hits[0].Snippet == "" {
		t.Fatalf("hit %+v", hits[0])
	}

	// limit
	hits, _ = runSearch("database", []string{"testagent"}, 1)
	if len(hits) != 1 {
		t.Fatalf("limit 1 got %d", len(hits))
	}

	// no match
	hits, _ = runSearch("missingxyz", []string{"testagent"}, 10)
	if len(hits) != 0 {
		t.Fatalf("no match want 0 got %d", len(hits))
	}

	// unknown agent error
	if _, err := runSearch("q", []string{"nope"}, 10); err == nil {
		t.Fatal("want error for unknown agent")
	}
}

func TestPrintSearchHuman(t *testing.T) {
	out := captureStdout(t, func() { printSearchHuman("q", nil) })
	if !strings.Contains(out, "No matches") {
		t.Fatalf("empty %q", out)
	}
	out = captureStdout(t, func() {
		printSearchHuman("test", []searchHit{{Agent: "gemini", ID: "a", Title: "t", Project: "p", Snippet: "snip", Matches: 1}})
	})
	if !strings.Contains(out, "1 match") || !strings.Contains(out, "gemini") {
		t.Fatalf("human %q", out)
	}
}
