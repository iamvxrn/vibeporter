package adapters

import (
	"time"

	"vibeporter/internal/models"
)

func ParseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func UnixMillisPtr(ms int64) *time.Time {
	if ms <= 0 {
		return nil
	}
	var t time.Time
	if ms < 1e12 {
		t = time.Unix(ms, 0)
	} else {
		t = time.UnixMilli(ms)
	}
	return &t
}

func CwdFromMeta(conv *models.Conversation) string {
	if conv == nil || conv.Metadata == nil {
		return ""
	}
	if s, ok := conv.Metadata["cwd"].(string); ok {
		return s
	}
	return ""
}
