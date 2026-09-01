// Package handoff coordinates local compaction, native session creation, and manifests.
package handoff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vibeporter/internal/adapters"
	"vibeporter/internal/compact"
	"vibeporter/internal/models"
)

type Options struct {
	SourceAgent string
	SourceID    string
	TargetAgent string
	TargetPath  string
	Budget      int
	Strategy    compact.Strategy
	DryRun      bool
	ManifestDir string
}

type Result struct {
	SourceAgent string           `json:"source_agent"`
	TargetAgent string           `json:"target_agent"`
	SourceID    string           `json:"source_id"`
	TargetPath  string           `json:"target_path,omitempty"`
	Strategy    compact.Strategy `json:"strategy"`
	Budget      int              `json:"budget_tokens"`
	Original    int              `json:"original_tokens_estimate"`
	Transferred int              `json:"transferred_tokens_estimate"`
	Kept        int              `json:"messages_kept"`
	Reduced     int              `json:"messages_reduced"`
}

func Prepare(source *models.Conversation, options Options) (*models.Conversation, Result, error) {
	if strings.TrimSpace(options.SourceAgent) == "" || strings.TrimSpace(options.TargetAgent) == "" {
		return nil, Result{}, fmt.Errorf("source and target agents are required")
	}
	// Reserve room for the explicit provenance header in the requested context budget.
	reserved := 96
	if options.Budget <= reserved {
		return nil, Result{}, fmt.Errorf("compact budget %d is too small for a handoff header", options.Budget)
	}
	conversation, report, err := compact.Compact(source, options.Budget-reserved, options.Strategy)
	if err != nil {
		return nil, Result{}, err
	}
	result := Result{SourceAgent: options.SourceAgent, TargetAgent: options.TargetAgent, SourceID: options.SourceID, Strategy: report.Strategy, Budget: options.Budget, Original: report.Original, Transferred: report.Result, Kept: report.Kept, Reduced: report.Reduced}
	header := models.NewMessage(models.RoleSystem, []models.Part{models.TextPart(fmt.Sprintf("This conversation was handed off from %s via Vibeporter.\nSource: %s\nStrategy: %s\nContext budget: %d tokens\nOriginal estimate: %d tokens~\nTransferred estimate: %d tokens~", options.SourceAgent, options.SourceID, report.Strategy, options.Budget, report.Original, report.Result))})
	conversation.Messages = append([]models.Message{header}, conversation.Messages...)
	result.Transferred = compact.EstimateTokens(conversation)
	if result.Transferred > options.Budget {
		return nil, Result{}, fmt.Errorf("compact budget %d is too small for handoff metadata", options.Budget)
	}
	return conversation, result, nil
}

func Execute(source *models.Conversation, injector adapters.Injector, options Options) (Result, error) {
	conversation, result, err := Prepare(source, options)
	if err != nil {
		return Result{}, err
	}
	if options.DryRun {
		return result, nil
	}
	target := strings.TrimSpace(options.TargetPath)
	if target == "" {
		defaults, ok := injector.(adapters.TargetDefaults)
		if !ok {
			return Result{}, fmt.Errorf("--target is required for %s", options.TargetAgent)
		}
		target, err = defaults.DefaultTarget(conversation)
		if err != nil {
			return Result{}, fmt.Errorf("default target: %w", err)
		}
	}
	written, err := injector.Inject(conversation, target)
	if err != nil {
		return Result{}, fmt.Errorf("injecting: %w", err)
	}
	result.TargetPath = written
	if err := writeManifest(result, options.ManifestDir); err != nil {
		return Result{}, err
	}
	return result, nil
}

func writeManifest(result Result, dir string) error {
	if strings.TrimSpace(dir) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("handoff manifest home: %w", err)
		}
		dir = filepath.Join(home, ".vibeporter", "handoffs")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	payload := struct {
		Result
		CreatedAt time.Time `json:"created_at"`
	}{result, time.Now().UTC()}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%d-%s.json", payload.CreatedAt.UnixNano(), sanitize(result.SourceID))
	return os.WriteFile(filepath.Join(dir, name), data, 0600)
}

func sanitize(value string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, value)
}
