package adapters

// CanonicalAgents is the one list of agent names vibeporter knows, shared by
// the CLI (internal/cmd) and the local web server (internal/web) so the two
// surfaces cannot drift into disagreeing about which agents exist.
//
// Before this, each surface kept its own literal copy, each checked by its
// own test against its own extractor/injector maps -- which caught a copy
// going out of sync with the maps in the SAME package, but nothing checked
// the two copies against each other. Adding an agent to one and forgetting
// the other would leave `vibeporter stats` listing eight agents and
// `vibeporter serve`'s UI listing seven (or the reverse), with both test
// suites green.
//
// Aliases (ag, kimi, dhs, wind) are deliberately excluded: they resolve to
// one of these and must never be enumerated alongside it, or a command that
// fans out over "every agent" would visit the same adapter twice.
var CanonicalAgents = []string{
	"antigravity", "claudecode", "cursor", "dsh",
	"gemini", "kimicode", "opencode", "windsurf",
}

// AgentAliases maps each alias to its canonical agent name.
var AgentAliases = map[string]string{
	"ag":   "antigravity",
	"kimi": "kimicode",
	"dhs":  "dsh",
	"wind": "windsurf",
}
