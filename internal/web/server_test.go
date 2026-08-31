package web

import (
	"strings"
	"testing"

	"vibeporter/internal/models"
)

func TestClipUnicode(t *testing.T) {
	s := clip("начало 💥 середина needle конец", "needle", 15)
	if !strings.Contains(s, "needle") || strings.Contains(s, "�") {
		t.Fatalf("invalid UTF-8 clipping: %q", s)
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
