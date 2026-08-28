package cmd

import (
	"vibeporter/internal/adapters"
	"vibeporter/internal/adapters/claudecode"
	"vibeporter/internal/adapters/cursor"
	"vibeporter/internal/adapters/dsh"
	"vibeporter/internal/adapters/gemini"
	"vibeporter/internal/adapters/kimicode"
	"vibeporter/internal/adapters/opencode"
)

var extractors = map[string]adapters.Extractor{
	"gemini":     gemini.NewAdapter(),
	"claudecode": claudecode.NewAdapter(),
	"opencode":   opencode.NewAdapter(),
	"kimicode":   kimicode.NewAdapter(),
	"kimi":       kimicode.NewAdapter(),
	"dsh":        dsh.NewAdapter(),
	"dhs":        dsh.NewAdapter(),
	"cursor":     cursor.NewAdapter(),
}

var injectors = map[string]adapters.Injector{
	"gemini":     gemini.NewAdapter(),
	"claudecode": claudecode.NewAdapter(),
	"opencode":   opencode.NewAdapter(),
	"kimicode":   kimicode.NewAdapter(),
	"kimi":       kimicode.NewAdapter(),
	"dsh":        dsh.NewAdapter(),
	"dhs":        dsh.NewAdapter(),
}

const supportedExtractors = "claudecode, opencode, gemini, kimicode (kimi), dsh (dhs), cursor"
const supportedInjectors = "claudecode, opencode, gemini, kimicode (kimi), dsh (dhs)"
