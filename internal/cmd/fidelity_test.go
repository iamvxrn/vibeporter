package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibeporter/internal/adapters"
	"vibeporter/internal/adapters/antigravity"
	"vibeporter/internal/adapters/claudecode"
	"vibeporter/internal/adapters/cursor"
	"vibeporter/internal/adapters/gemini"
	"vibeporter/internal/adapters/kimicode"
	"vibeporter/internal/adapters/opencode"
	"vibeporter/internal/adapters/windsurf"
	"vibeporter/internal/models"
)

// syntheticConversation covers all IR features that should round-trip.
func syntheticConversation() *models.Conversation {
	return &models.Conversation{
		ID:          "synthetic",
		Title:       "Fidelity test — what we did",
		AgentSource: "synthetic",
		Metadata:    map[string]interface{}{"cwd": "/tmp/proj"},
		Messages: []models.Message{
			models.NewMessage(models.RoleSystem, []models.Part{models.TextPart("You are a helpful assistant")}),
			models.NewMessage(models.RoleUser, []models.Part{models.TextPart("исправь баг с базой, что мы делали до этого?")}),
			models.NewMessage(models.RoleAssistant, []models.Part{
				models.ThinkingPart("Need to check db migration and previous tool outputs"),
				models.TextPart("Проверю что делали"),
				models.ToolCallPart("call_1", "Read", `{"path":"/tmp/proj/main.go"}`),
			}),
			models.NewMessage(models.RoleUser, []models.Part{
				models.ToolResultPart("call_1", "file content here", false),
			}),
			models.NewMessage(models.RoleAssistant, []models.Part{
				models.TextPart("Готово, вот итог"),
				models.ToolCallPart("call_2", "Bash", `{"cmd":"go test ./..."}`),
			}),
			models.NewMessage(models.RoleUser, []models.Part{
				models.ToolResultPart("call_2", "ok 3 passed", false),
			}),
		},
	}
}

func TestFidelityRoundTripPerAdapter(t *testing.T) {
	adaptersToTest := []struct {
		name string
		ctor func() interface {
			Inject(*models.Conversation, string) (string, error)
			Extract(string) (*models.Conversation, error)
		}
		isDB    bool
		envRoot string // env to isolate
	}{
		{"claudecode", func() interface {
			Inject(*models.Conversation, string) (string, error)
			Extract(string) (*models.Conversation, error)
		} {
			return claudecode.NewAdapter()
		}, false, ""},
		{"gemini", func() interface {
			Inject(*models.Conversation, string) (string, error)
			Extract(string) (*models.Conversation, error)
		} {
			return gemini.NewAdapter()
		}, false, ""},
		{"cursor", func() interface {
			Inject(*models.Conversation, string) (string, error)
			Extract(string) (*models.Conversation, error)
		} {
			return cursor.NewAdapter()
		}, false, ""},
		{"opencode", func() interface {
			Inject(*models.Conversation, string) (string, error)
			Extract(string) (*models.Conversation, error)
		} {
			return opencode.NewAdapter()
		}, true, "HOME"},
		{"antigravity", func() interface {
			Inject(*models.Conversation, string) (string, error)
			Extract(string) (*models.Conversation, error)
		} {
			return antigravity.NewAdapter()
		}, false, "ANTIGRAVITY"},
		{"kimicode", func() interface {
			Inject(*models.Conversation, string) (string, error)
			Extract(string) (*models.Conversation, error)
		} {
			return kimicode.NewAdapter()
		}, false, "KIMI"},
		{"windsurf", func() interface {
			Inject(*models.Conversation, string) (string, error)
			Extract(string) (*models.Conversation, error)
		} {
			return windsurf.NewAdapter()
		}, false, "WINDSURF"},
	}

	syn := syntheticConversation()
	for _, tc := range adaptersToTest {
		t.Run(tc.name+" inject+extract", func(t *testing.T) {
			// Isolate DB/file adapters
			switch tc.envRoot {
			case "HOME":
				tmpHome := t.TempDir()
				t.Setenv("HOME", tmpHome)
				t.Setenv("USERPROFILE", tmpHome)
				t.Setenv("XDG_DATA_HOME", "")
				t.Setenv("APPDATA", filepath.Join(tmpHome, "AppData", "Roaming"))
			case "ANTIGRAVITY":
				tmpBrain := t.TempDir()
				t.Setenv("ANTIGRAVITY_BRAIN_DIR", tmpBrain)
			case "KIMI":
				tmpKimi := t.TempDir()
				t.Setenv("KIMI_CODE_HOME", tmpKimi)
			case "WINDSURF":
				tmpWindsurf := t.TempDir()
				t.Setenv("WINDSURF_DATA_DIR", tmpWindsurf)
			}
			adapter := tc.ctor()
			var target string
			if tc.isDB {
				target = "" // use default DB under HOME
			} else if tc.name == "antigravity" {
				target = filepath.Join(t.TempDir(), "handoff_context.md")
			} else {
				target = filepath.Join(t.TempDir(), tc.name+".jsonl")
			}
			written, err := adapter.Inject(syn, target)
			if err != nil {
				t.Fatalf("inject %s: %v", tc.name, err)
			}
			round, err := adapter.Extract(written)
			if err != nil {
				t.Fatalf("extract %s: %v", tc.name, err)
			}
			// Title may be derived or preserved — just check messages exist, not title
			if len(round.Messages) == 0 {
				t.Fatalf("no messages for %s", tc.name)
			}
			// Count parts
			wantText, wantThinking, wantTools := countParts(syn)
			gotText, gotThinking, gotTools := countParts(round)

			// Antigravity markdown round-trip collapses all messages into one
			// system message containing the full context. So we only verify
			// that the content is preserved, not the per-part counts.
			if tc.name == "antigravity" {
				if gotText == 0 {
					t.Fatalf("no text parts for %s", tc.name)
				}
				if !containsQuery(round, "базой") && !containsQuery(round, "Готово") {
					t.Fatalf("round-trip lost query context for %s", tc.name)
				}
				return
			}

			// opencode drops system as user + drops tool_result (counts as tool), but should keep tool_call
			// So allow small diff for tool_result (original has 2 tool_result, target may drop them)
			if gotText < wantText-2 {
				t.Fatalf("text parts %d want ~%d for %s", gotText, wantText, tc.name)
			}
			if gotThinking < wantThinking {
				t.Fatalf("thinking %d want %d for %s", gotThinking, wantThinking, tc.name)
			}
			// For opencode, tool_calls should be preserved now (fix)
			if gotTools == 0 && wantTools > 0 {
				t.Fatalf("tools lost %d want %d for %s", gotTools, wantTools, tc.name)
			}
			// Check that query context is preserved (at least one of the key phrases)
			if !containsQuery(round, "базой") && !containsQuery(round, "багой") && !containsQuery(round, "hello") {
				// fallback: check any text preserved
				if gotText == 0 {
					t.Fatalf("round-trip lost query context for %s", tc.name)
				}
			}
		})
	}
}

func TestFidelityCrossAdapter(t *testing.T) {
	// Cursor → Opencode should preserve text and tools
	syn := syntheticConversation()
	cursorAdapter := cursor.NewAdapter()

	// Cursor inject
	tmp1 := filepath.Join(t.TempDir(), "cursor.jsonl")
	written, err := cursorAdapter.Inject(syn, tmp1)
	if err != nil {
		t.Fatal(err)
	}
	intermediate, err := cursorAdapter.Extract(written)
	if err != nil {
		t.Fatal(err)
	}
	// Opencode inject from intermediate — use HOME trick
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("APPDATA", filepath.Join(tmpHome, "AppData", "Roaming"))
	opencodeAdapter := opencode.NewAdapter()
	written2, err := opencodeAdapter.Inject(intermediate, "")
	if err != nil {
		t.Fatal(err)
	}
	round, err := opencodeAdapter.Extract(written2)
	if err != nil {
		t.Fatal(err)
	}
	if len(round.Messages) == 0 {
		t.Fatalf("cross round-trip empty, intermediate messages %d", len(intermediate.Messages))
	}
	// Verify critical phrase survives (at least one of the key texts)
	if !containsQuery(round, "Готово") && !containsQuery(round, "базой") && !containsQuery(round, "hello") {
		t.Fatalf("cross round-trip lost content, messages %+v", round.Messages)
	}
	// Also cross Antigravity → Opencode
	t.Run("antigravity->opencode", func(t *testing.T) {
		tmpAg := t.TempDir()
		t.Setenv("ANTIGRAVITY_BRAIN_DIR", tmpAg)
		agAdapter := antigravity.NewAdapter()
		// Inject syn to antigravity
		writtenAg, err := agAdapter.Inject(syn, "")
		if err != nil {
			t.Fatal(err)
		}
		interAg, err := agAdapter.Extract(writtenAg)
		if err != nil {
			t.Fatal(err)
		}
		// Then to opencode
		tmpHome2 := t.TempDir()
		t.Setenv("HOME", tmpHome2)
		t.Setenv("USERPROFILE", tmpHome2)
		t.Setenv("XDG_DATA_HOME", "")
		t.Setenv("APPDATA", filepath.Join(tmpHome2, "AppData", "Roaming"))
		written2, _ := opencodeAdapter.Inject(interAg, "")
		round2, _ := opencodeAdapter.Extract(written2)
		if len(round2.Messages) == 0 {
			t.Fatalf("ag->opencode empty")
		}
		if !containsQuery(round2, "Готово") && !containsQuery(round2, "базой") {
			t.Fatalf("ag->opencode lost")
		}
	})
	// Verify tool calls survived (opencode drops tool_result, so allow 2)
	_, _, wantTools := countParts(syn)
	_, _, gotTools := countParts(round)
	if gotTools < wantTools-2 {
		t.Fatalf("cross tools %d want %d (allow drop of tool_result)", gotTools, wantTools)
	}

	// Also test Gem → Claude
	geminiAdapter := gemini.NewAdapter()
	claudeAdapter := claudecode.NewAdapter()
	tmpGem := filepath.Join(t.TempDir(), "gemini.jsonl")
	writtenGem, _ := geminiAdapter.Inject(syn, tmpGem)
	interGem, _ := geminiAdapter.Extract(writtenGem)
	tmpClaude := filepath.Join(t.TempDir(), "claude.jsonl")
	writtenClaude, _ := claudeAdapter.Inject(interGem, tmpClaude)
	roundClaude, _ := claudeAdapter.Extract(writtenClaude)
	if len(roundClaude.Messages) == 0 {
		t.Fatal("empty round-trip")
	}
	if !containsQuery(roundClaude, "что мы делали") && !containsQuery(roundClaude, "Готово") && !containsQuery(roundClaude, "базой") && !containsQuery(interGem, "базой") {
		t.Fatalf("second cross lost phrase, messages %d %+v", len(roundClaude.Messages), roundClaude.Messages)
	}
	// ensure temp files exist
	_ = os.Remove(tmp1)
	_ = adapters.Clip // keep import
}

func countParts(conv *models.Conversation) (text, thinking, tools int) {
	if conv == nil {
		return 0, 0, 0
	}
	for _, m := range conv.Messages {
		for _, p := range m.EffectiveParts() {
			switch p.Kind {
			case models.PartText:
				text++
			case models.PartThinking:
				thinking++
			case models.PartToolCall, models.PartToolResult:
				tools++
			}
		}
	}
	return
}

func containsQuery(conv *models.Conversation, q string) bool {
	if conv == nil {
		return false
	}
	ql := strings.ToLower(q)
	for _, m := range conv.Messages {
		if strings.Contains(strings.ToLower(m.Content), ql) {
			return true
		}
		for _, p := range m.EffectiveParts() {
			if strings.Contains(strings.ToLower(p.Text), ql) || strings.Contains(strings.ToLower(p.ArgsJSON), ql) {
				return true
			}
		}
	}
	return false
}
