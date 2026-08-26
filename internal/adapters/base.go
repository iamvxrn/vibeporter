package adapters

import (
	"time"

	"vibeporter/internal/models"
)

// ChatInfo is one row in `vibeporter list`. Path is the value Extract expects
// (a file path, or a session id for SQLite-backed agents).
type ChatInfo struct {
	ID        string
	Path      string
	Agent     string
	Title     string
	Project   string
	UpdatedAt time.Time
}

type Extractor interface {
	Extract(sourcePath string) (*models.Conversation, error)
	ListConversations() ([]ChatInfo, error)
}

type Injector interface {
	Inject(conversation *models.Conversation, targetPath string) (string, error)
}
