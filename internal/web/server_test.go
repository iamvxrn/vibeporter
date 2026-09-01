package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vibeporter/internal/adapters"
	"vibeporter/internal/models"
)

func TestClipUnicode(t *testing.T) {
	s := clip("начало 💥 середина needle конец", "needle", 15)
	if !strings.Contains(s, "needle") || strings.Contains(s, "�") {
		t.Fatalf("invalid UTF-8 clipping: %q", s)
	}
}

type handoffExtractor struct{}

func (handoffExtractor) ListConversations() ([]adapters.ChatInfo, error) {
	return []adapters.ChatInfo{{ID: "chat", Path: "path"}}, nil
}
func (handoffExtractor) Extract(string) (*models.Conversation, error) {
	return &models.Conversation{ID: "chat", Messages: []models.Message{{Role: models.RoleUser, Content: "task"}}}, nil
}

type handoffInjector struct{ calls int }

func (i *handoffInjector) DefaultTarget(*models.Conversation) (string, error) { return "target", nil }
func (i *handoffInjector) Inject(*models.Conversation, string) (string, error) {
	i.calls++
	return "created", nil
}

func TestHandoffPreviewDoesNotInjectAndHandoffInjects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	oldExtractors, oldInjectors := extractors, injectors
	t.Cleanup(func() { extractors, injectors = oldExtractors, oldInjectors })
	injector := &handoffInjector{}
	extractors = map[string]adapters.Extractor{"source": handoffExtractor{}}
	injectors = map[string]adapters.Injector{"target": injector}
	body := []byte(`{"from":"source","to":"target","source":"chat","compact":"200"}`)
	preview := httptest.NewRecorder()
	handleHandoffPreview(preview, httptest.NewRequest(http.MethodPost, "/api/handoff/preview", bytes.NewReader(body)))
	if preview.Code != http.StatusOK || injector.calls != 0 {
		t.Fatalf("preview status=%d calls=%d", preview.Code, injector.calls)
	}
	create := httptest.NewRecorder()
	handleHandoff(create, httptest.NewRequest(http.MethodPost, "/api/handoff", bytes.NewReader(body)))
	if create.Code != http.StatusOK || injector.calls != 1 {
		t.Fatalf("handoff status=%d calls=%d", create.Code, injector.calls)
	}
}

func TestComparePartsIncludesToolArgumentsAndResults(t *testing.T) {
	from := &models.Conversation{Messages: []models.Message{{Parts: []models.Part{
		models.ToolCallPart("1", "run", `{"cmd":"one"}`),
		models.ToolResultPart("1", "result one", false),
	}}}}
	to := &models.Conversation{Messages: []models.Message{{Parts: []models.Part{
		models.ToolCallPart("1", "run", `{"cmd":"two"}`),
		models.ToolResultPart("1", "result two", false),
	}}}}

	diff := compareParts(from, to)
	if diff.Equal || len(diff.Mismatches) != 2 {
		t.Fatalf("parts diff = %+v", diff)
	}
	if diff.Mismatches[0].From.ArgsJSON == diff.Mismatches[0].To.ArgsJSON || diff.Mismatches[1].From.Text == diff.Mismatches[1].To.Text {
		t.Fatalf("tool args/results were not compared: %+v", diff.Mismatches)
	}
}
