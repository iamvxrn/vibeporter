package adapters

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"vibeporter/internal/models"
)

// NewUUID returns a random RFC 4122 v4 UUID.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("vp-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// NewPrefixedID returns prefix + 26 hex chars (OpenCode-style ids).
func NewPrefixedID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b[:])[:26]
}

func EnsureMeta(conv *models.Conversation) map[string]interface{} {
	if conv.Metadata == nil {
		conv.Metadata = map[string]interface{}{}
	}
	return conv.Metadata
}
