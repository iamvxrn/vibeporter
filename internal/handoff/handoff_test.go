package handoff

import (
	"testing"
	"vibeporter/internal/adapters"
	"vibeporter/internal/compact"
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
}

type countingInjector struct{ calls int }

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
