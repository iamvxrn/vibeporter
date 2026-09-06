// Package handoff coordinates local compaction, native session creation, and context packets.
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
	"vibeporter/internal/contextpacket"
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
	SourceAgent string                `json:"source_agent"`
	TargetAgent string                `json:"target_agent"`
	SourceID    string                `json:"source_id"`
	TargetPath  string                `json:"target_path,omitempty"`
	Strategy    compact.Strategy      `json:"strategy"`
	Budget      int                   `json:"budget_tokens"`
	Original    int                   `json:"original_tokens_estimate"`
	Transferred int                   `json:"transferred_tokens_estimate"`
	Kept        int                   `json:"messages_kept"`
	Reduced     int                   `json:"messages_reduced"`
	PacketPath  string                `json:"packet_path,omitempty"`
	Packet      *contextpacket.Packet `json:"packet,omitempty"`
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
	packet := contextpacket.FromSelection(conversation, contextpacket.Selection{
		SourceAgent: options.SourceAgent,
		SourceID:    options.SourceID,
		TargetAgent: options.TargetAgent,
		TargetPath:  options.TargetPath,
		Strategy:    string(report.Strategy),
		Budget:      options.Budget,
		Original:    report.Original,
		Transferred: report.Result,
		Kept:        report.Kept,
		Reduced:     report.Reduced,
	})
	result.Packet = &packet
	header := models.NewMessage(models.RoleSystem, []models.Part{models.TextPart(fmt.Sprintf("This context packet was delivered from %s via Vibeporter.\nSource session: %s\nStrategy: %s\nContext budget: %d tokens\nOriginal estimate: %d tokens~\nTransferred estimate: %d tokens~", options.SourceAgent, options.SourceID, report.Strategy, options.Budget, report.Original, report.Result))})
	conversation.Messages = append([]models.Message{header}, conversation.Messages...)
	result.Transferred = compact.EstimateTokens(conversation)
	if result.Packet != nil {
		result.Packet.SelectedContext.TransferredTokensEstimate = result.Transferred
	}
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
	if result.Packet != nil && len(result.Packet.Handoffs) > 0 {
		result.Packet.Handoffs[0].Path = written
	}
	path, err := writePacket(result, options.ManifestDir)
	if err != nil {
		return Result{}, err
	}
	result.PacketPath = path
	return result, nil
}

func writePacket(result Result, dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("context packet home: %w", err)
		}
		dir = filepath.Join(home, ".vibeporter", "handoffs")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	created := time.Now().UTC()
	packet := result.Packet
	if packet == nil {
		p := contextpacket.FromSelection(nil, contextpacket.Selection{
			SourceAgent: result.SourceAgent,
			SourceID:    result.SourceID,
			TargetAgent: result.TargetAgent,
			TargetPath:  result.TargetPath,
			Strategy:    string(result.Strategy),
			Budget:      result.Budget,
			Original:    result.Original,
			Transferred: result.Transferred,
			Kept:        result.Kept,
			Reduced:     result.Reduced,
			CreatedAt:   created,
		})
		packet = &p
	} else {
		packet.CreatedAt = created
		if packet.SelectedContext.TransferredTokensEstimate == 0 {
			packet.SelectedContext.TransferredTokensEstimate = result.Transferred
		}
	}
	payload := struct {
		contextpacket.Packet
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
	}{
		Packet:      *packet,
		SourceAgent: result.SourceAgent,
		TargetAgent: result.TargetAgent,
		SourceID:    result.SourceID,
		TargetPath:  result.TargetPath,
		Strategy:    result.Strategy,
		Budget:      result.Budget,
		Original:    result.Original,
		Transferred: result.Transferred,
		Kept:        result.Kept,
		Reduced:     result.Reduced,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d-%s.json", created.UnixNano(), sanitize(result.SourceID))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func sanitize(value string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, value)
}
