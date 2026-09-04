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

// canonicalAgents is the deduplicated set of agent names, one entry per
// adapter. Aliases (ag, kimi, dhs, wind) resolve to these and must never be
// enumerated alongside them, or commands that fan out over every agent (stats,
// search) would visit the same adapter twice and double-count it.
// TestRegistryAliasesAreCanonical keeps this in sync with the maps above.
var canonicalAgents = []string{
	"antigravity", "claudecode", "cursor", "dsh",
	"gemini", "kimicode", "opencode", "windsurf",
}

// agentAliases maps each alias to its canonical agent name.
var agentAliases = map[string]string{
	"ag":   "antigravity",
	"kimi": "kimicode",
	"dhs":  "dsh",
	"wind": "windsurf",
}

const supportedExtractors = "claudecode, opencode, gemini, antigravity (ag), kimicode (kimi), dsh (dhs), cursor, windsurf (wind)"
const supportedInjectors = "claudecode, opencode, gemini, antigravity (ag), kimicode (kimi), dsh (dhs), cursor, windsurf (wind)"
