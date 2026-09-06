package contextpacket

import (
	"testing"
	"time"

	"vibeporter/internal/models"
)

func TestFromSelectionFillsProvenanceWithoutInventingDecisions(t *testing.T) {
	conv := &models.Conversation{
		Title:    "Auth rewrite",
		Metadata: map[string]interface{}{"cwd": "/work/api"},
		Messages: []models.Message{
			{Role: models.RoleUser, Content: "keep sessions local"},
			{Role: models.RoleAssistant, Content: "using sqlite on disk"},
		},
	}
	got := FromSelection(conv, Selection{
		SourceAgent: "claudecode",
		SourceID:    "abc",
		TargetAgent: "opencode",
		TargetPath:  "/tmp/out",
		Strategy:    "smart",
		Budget:      2000,
		Original:    9000,
		Transferred: 1800,
		Kept:        2,
		Reduced:     40,
		CreatedAt:   time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC),
	})
	if got.Kind != Kind || got.Version != Version {
		t.Fatalf("identity: %+v", got)
	}
	if got.Title != "Auth rewrite" || got.Project != "/work/api" {
		t.Fatalf("project: %+v", got)
	}
	if got.CurrentState != "using sqlite on disk" {
		t.Fatalf("state %q", got.CurrentState)
	}
	if len(got.Decisions) != 0 || len(got.Constraints) != 0 || len(got.OpenQuestions) != 0 || len(got.Artifacts) != 0 {
		t.Fatalf("must not invent structured fields: %+v", got)
	}
	if len(got.SourceSessions) != 1 || got.SourceSessions[0].Agent != "claudecode" {
		t.Fatalf("source: %+v", got.SourceSessions)
	}
	if len(got.Handoffs) != 1 || got.Handoffs[0].Kind != "native_session" {
		t.Fatalf("handoff: %+v", got.Handoffs)
	}
}
