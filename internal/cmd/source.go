package cmd

import (
	"fmt"
	"os"
	"strings"

	"vibeporter/internal/adapters"
)

// resolveSource maps a list ID (or unique prefix) to the Extract() argument.
// Existing file paths pass through unchanged.
func resolveSource(extractor adapters.Extractor, spec string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", fmt.Errorf("empty source")
	}
	if st, err := os.Stat(spec); err == nil && !st.IsDir() {
		return spec, nil
	}

	chats, err := extractor.ListConversations()
	if err != nil {
		return spec, nil
	}

	var prefixed []adapters.ChatInfo
	for _, c := range chats {
		if c.ID == spec || c.Path == spec {
			return c.Path, nil
		}
		if strings.HasPrefix(c.ID, spec) {
			prefixed = append(prefixed, c)
		}
	}
	if len(prefixed) == 1 {
		return prefixed[0].Path, nil
	}
	if len(prefixed) > 1 {
		return "", fmt.Errorf("ambiguous source %q matches %d chats; pass the full id", spec, len(prefixed))
	}
	return spec, nil
}
