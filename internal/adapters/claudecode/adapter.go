package claudecode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vibeporter/internal/models"
)

type Adapter struct{}

func NewAdapter() *Adapter {
	return &Adapter{}
}

func (a *Adapter) Extract(sourcePath string) (*models.Conversation, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	id := filepath.Base(sourcePath)
	id = strings.TrimSuffix(id, ".jsonl")

	conv := &models.Conversation{
		ID:          id,
		AgentSource: "claudecode",
		Messages:    []models.Message{},
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal(line, &data); err != nil {
			continue // skip invalid json
		}

		msgType, _ := data["type"].(string)
		if msgType != "user" && msgType != "assistant" {
			continue // skip system, tool_result, etc. for now unless needed
		}

		role := models.RoleSystem
		if msgType == "user" {
			role = models.RoleUser
		} else if msgType == "assistant" {
			role = models.RoleAssistant
		}

		messageObj, ok := data["message"].(map[string]interface{})
		if !ok {
			continue
		}

		contentArr, ok := messageObj["content"].([]interface{})
		if !ok {
			continue
		}

		var textContent strings.Builder
		for _, part := range contentArr {
			partMap, ok := part.(map[string]interface{})
			if !ok {
				continue
			}

			if partType, _ := partMap["type"].(string); partType == "text" {
				if text, ok := partMap["text"].(string); ok {
					textContent.WriteString(text)
					textContent.WriteString("\n")
				}
			} else if partType == "tool_use" {
				toolName, _ := partMap["name"].(string)
				textContent.WriteString(fmt.Sprintf("[Tool Use: %s]\n", toolName))
			}
		}

		if textContent.Len() > 0 {
			conv.Messages = append(conv.Messages, models.Message{
				Role:    role,
				Content: strings.TrimSpace(textContent.String()),
			})
		}
	}

	return conv, scanner.Err()
}
