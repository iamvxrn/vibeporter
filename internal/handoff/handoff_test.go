package handoff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibeporter/internal/adapters"
	"vibeporter/internal/compact"
	"vibeporter/internal/contextpacket"
	"vibeporter/internal/models"
)

func TestPrepareAddsHeaderWithoutMutatingSource(t *testing.T) {
	source := &models.Conversation{Messages: []models.Message{{Role: models.RoleUser, Content: "задача"}, {Role: models.RoleAssistant, Content: "готово"}}}
	got, result, err := Prepare(source, Options{SourceAgent: "cursor", SourceID: "abc", TargetAgent: "opencode", Budget: 200, Strategy: compact.Smart})
	if err != nil || result.Transferred > 200 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(source.Messages) != 2 || got.Messages[0].Role != models.RoleSystem {
		t.Fatalf("source/header invalid")
	}
	if !strings.Contains(got.Messages[0].Content, "context packet") {
		t.Fatalf("header: %q", got.Messages[0].Content)
	}
	if result.Packet == nil || result.Packet.Kind != contextpacket.Kind {
		t.Fatalf("packet: %+v", result.Packet)
	}
}

func TestExecuteWritesContextPacket(t *testing.T) {
	dir := t.TempDir()
	injector := &countingInjector{}
	result, err := Execute(&models.Conversation{
		Title:    "task",
		Metadata: map[string]interface{}{"cwd": "/proj"},
		Messages: []models.Message{{Role: models.RoleUser, Content: "task"}},
	}, injector, Options{SourceAgent: "cursor", SourceID: "a", TargetAgent: "opencode", Budget: 200, Strategy: compact.Smart, ManifestDir: dir})
	if err != nil || injector.calls != 1 || result.PacketPath == "" {
		t.Fatalf("err=%v calls=%d path=%q", err, injector.calls, result.PacketPath)
	}
	raw, err := os.ReadFile(result.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["kind"] != contextpacket.Kind || doc["source_agent"] != "cursor" {
		t.Fatalf("doc=%s", raw)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(matches) != 1 {
		t.Fatalf("files: %v", matches)
	}
}

type countingInjector struct{ calls int }

func (i *countingInjector) DefaultTarget(*models.Conversation) (string, error) { return "target", nil }

func (i *countingInjector) Inject(*models.Conversation, string) (string, error) {
	i.calls++
	return "target", nil
}
func TestDryRunDoesNotInject(t *testing.T) {
	injector := &countingInjector{}
	_, err := Execute(&models.Conversation{Messages: []models.Message{{Role: models.RoleUser, Content: "task"}}}, injector, Options{SourceAgent: "cursor", SourceID: "a", TargetAgent: "opencode", Budget: 200, Strategy: compact.Smart, DryRun: true})
	if err != nil || injector.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, injector.calls)
	}
	var _ adapters.Injector = injector
}
