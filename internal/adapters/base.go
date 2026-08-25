package adapters

import "vibeporter/internal/models"

type ChatInfo struct {
	ID    string
	Path  string
	Agent string
}

type Extractor interface {
	Extract(sourcePath string) (*models.Conversation, error)
	ListConversations() ([]ChatInfo, error)
}

type Injector interface {
	Inject(conversation *models.Conversation, targetPath string) (string, error)
}
