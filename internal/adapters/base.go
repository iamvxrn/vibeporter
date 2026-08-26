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

// TargetDefaults can pick a native on-disk (or store) location when migrate
// is invoked without --target.
type TargetDefaults interface {
	DefaultTarget(conversation *models.Conversation) (string, error)
}
