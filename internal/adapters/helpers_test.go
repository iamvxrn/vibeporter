package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vibeporter/internal/models"
)

func TestNewUUIDAndPrefixedID(t *testing.T) {
	id := NewUUID()
	if strings.Count(id, "-") != 4 || len(id) != 36 {
		t.Fatalf("uuid: %q", id)
	}
	prefixed := NewPrefixedID("ses_")
	if !strings.HasPrefix(prefixed, "ses_") || len(prefixed) != 4+26 {
		t.Fatalf("prefixed: %q", prefixed)
	}
}

func TestEnsureMeta(t *testing.T) {
	conv := &models.Conversation{}
	m := EnsureMeta(conv)
	if conv.Metadata == nil || m == nil {
		t.Fatal("expected metadata map")
	}
	conv.Metadata["cwd"] = "/x"
	if EnsureMeta(conv)["cwd"] != "/x" {
		t.Fatal("should keep existing metadata")
	}
}

func TestParseTime(t *testing.T) {
	if ParseTime("") != nil {
		t.Fatal("empty")
	}
	got := ParseTime("2026-01-02T03:04:05Z")
	if got == nil || got.Year() != 2026 {
		t.Fatalf("rfc3339: %v", got)
	}
	if ParseTime("not-a-time") != nil {
		t.Fatal("garbage")
	}
}

func TestUnixMillisPtr(t *testing.T) {
	if UnixMillisPtr(0) != nil {
		t.Fatal("zero")
	}
	sec := UnixMillisPtr(1_700_000_000)
	if sec == nil || !sec.Equal(time.Unix(1_700_000_000, 0)) {
		t.Fatalf("seconds: %v", sec)
	}
	ms := UnixMillisPtr(1_700_000_000_000)
	if ms == nil || !ms.Equal(time.UnixMilli(1_700_000_000_000)) {
		t.Fatalf("millis: %v", ms)
	}
}

func TestCwdFromMeta(t *testing.T) {
	if CwdFromMeta(nil) != "" {
		t.Fatal("nil conv")
	}
	if CwdFromMeta(&models.Conversation{}) != "" {
		t.Fatal("nil metadata")
	}
	conv := &models.Conversation{Metadata: map[string]interface{}{"cwd": "/home/a"}}
	if CwdFromMeta(conv) != "/home/a" {
		t.Fatal("cwd")
	}
	if CwdFromMeta(&models.Conversation{Metadata: map[string]interface{}{"cwd": 1}}) != "" {
		t.Fatal("non-string cwd")
	}
}

func TestForEachJSONLLimited(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.jsonl")
	body := "{\"a\":1}\nnot json\n{\"b\":2}\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var keys []string
	if err := ForEachJSONLLimited(p, 1024, 1024, func(m map[string]interface{}) {
		for k := range m {
			keys = append(keys, k)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("keys=%v", keys)
	}

	empty := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ForEachJSONLLimited(empty, 10, 10, func(map[string]interface{}) {
		t.Fatal("empty file should yield no records")
	}); err != nil {
		t.Fatal(err)
	}

	if err := ForEachJSONLLimited(filepath.Join(dir, "missing"), 10, 10, func(map[string]interface{}) {}); err == nil {
		t.Fatal("expected missing file error")
	}

	big := filepath.Join(dir, "big.jsonl")
	var bigBody strings.Builder
	for i := 0; i < 40; i++ {
		bigBody.WriteString(`{"n":`)
		bigBody.WriteString(string(rune('0' + (i % 10))))
		bigBody.WriteByte('}')
		bigBody.WriteByte('\n')
	}
	if err := os.WriteFile(big, []byte(bigBody.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	n := 0
	if err := ForEachJSONLLimited(big, 20, 20, func(map[string]interface{}) { n++ }); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("head+tail should yield some records")
	}
	if err := ForEachJSONLLimited(p, -1, -1, func(map[string]interface{}) {}); err != nil {
		t.Fatal(err)
	}
}

func TestClipEdges(t *testing.T) {
	if got := Clip("ab", 0); got != "ab" {
		t.Fatalf("n=0: %q", got)
	}
	if got := Clip("ab", 1); got != "…" {
		t.Fatalf("n=1: %q", got)
	}
}
