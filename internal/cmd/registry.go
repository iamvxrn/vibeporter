package cmd

import (
	"vibeporter/internal/adapters"
	"vibeporter/internal/adapters/antigravity"
	"vibeporter/internal/adapters/claudecode"
	"vibeporter/internal/adapters/cursor"
	"vibeporter/internal/adapters/dsh"
	"vibeporter/internal/adapters/gemini"
	"vibeporter/internal/adapters/kimicode"
	"vibeporter/internal/adapters/opencode"
	"vibeporter/internal/adapters/windsurf"
)

var extractors = map[string]adapters.Extractor{
	"gemini":      gemini.NewAdapter(),
	"antigravity": antigravity.NewAdapter(),
	"ag":          antigravity.NewAdapter(),
	"claudecode":  claudecode.NewAdapter(),
	"opencode":    opencode.NewAdapter(),
	"kimicode":    kimicode.NewAdapter(),
	"kimi":        kimicode.NewAdapter(),
	"dsh":         dsh.NewAdapter(),
	"dhs":         dsh.NewAdapter(),
	"cursor":      cursor.NewAdapter(),
	"windsurf":    windsurf.NewAdapter(),
	"wind":        windsurf.NewAdapter(),
}

var injectors = map[string]adapters.Injector{
	"gemini":      gemini.NewAdapter(),
	"antigravity": antigravity.NewAdapter(),
	"ag":          antigravity.NewAdapter(),
	"claudecode":  claudecode.NewAdapter(),
	"opencode":    opencode.NewAdapter(),
	"kimicode":    kimicode.NewAdapter(),
	"kimi":        kimicode.NewAdapter(),
	"dsh":         dsh.NewAdapter(),
	"dhs":         dsh.NewAdapter(),
	"cursor":      cursor.NewAdapter(),
	"windsurf":    windsurf.NewAdapter(),
	"wind":        windsurf.NewAdapter(),
}

// canonicalAgents and agentAliases are adapters.CanonicalAgents and
// adapters.AgentAliases -- see there for why this is one shared list rather
// than a copy per package. TestRegistryAliasesAreCanonical keeps this in
// sync with the maps above.
var canonicalAgents = adapters.CanonicalAgents

var agentAliases = adapters.AgentAliases

const supportedExtractors = "claudecode, opencode, gemini, antigravity (ag), kimicode (kimi), dsh (dhs), cursor, windsurf (wind)"
const supportedInjectors = "claudecode, opencode, gemini, antigravity (ag), kimicode (kimi), dsh (dhs), cursor, windsurf (wind)"
