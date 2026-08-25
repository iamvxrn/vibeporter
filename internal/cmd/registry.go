package cmd

import (
	"vibeporter/internal/adapters"
	"vibeporter/internal/adapters/claudecode"
	"vibeporter/internal/adapters/gemini"
	"vibeporter/internal/adapters/opencode"
)

var extractors = map[string]adapters.Extractor{
	"gemini":     gemini.NewAdapter(),
	"claudecode": claudecode.NewAdapter(),
	"opencode":   opencode.NewAdapter(),
}

var injectors = map[string]adapters.Injector{
	"gemini": gemini.NewAdapter(),
}
