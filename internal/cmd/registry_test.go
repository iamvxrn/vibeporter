package cmd

import (
	"sort"
	"testing"
)

// Every registry key must be either a canonical agent or a declared alias.
// Without this, adding an adapter under a new alias silently makes commands
// that fan out over all agents (stats, search) visit it twice and double-count.
func TestRegistryAliasesAreCanonical(t *testing.T) {
	canonical := map[string]bool{}
	for _, a := range canonicalAgents {
		canonical[a] = true
	}
	for _, m := range []map[string]bool{keys(extractorKeys()), keys(injectorKeys())} {
		for k := range m {
			if canonical[k] {
				continue
			}
			target, ok := agentAliases[k]
			if !ok {
				t.Errorf("registry key %q is neither in canonicalAgents nor agentAliases", k)
				continue
			}
			if !canonical[target] {
				t.Errorf("alias %q points at %q which is not a canonical agent", k, target)
			}
		}
	}
	for _, a := range canonicalAgents {
		if _, ok := extractors[a]; !ok {
			t.Errorf("canonical agent %q has no extractor", a)
		}
		if _, ok := injectors[a]; !ok {
			t.Errorf("canonical agent %q has no injector", a)
		}
	}
}

func extractorKeys() []string {
	var out []string
	for k := range extractors {
		out = append(out, k)
	}
	return out
}

func injectorKeys() []string {
	var out []string
	for k := range injectors {
		out = append(out, k)
	}
	return out
}

func keys(in []string) map[string]bool {
	m := map[string]bool{}
	for _, k := range in {
		m[k] = true
	}
	return m
}

// targetAgents drives `stats` and unfiltered `search`. Enumerating an alias
// alongside its canonical name made both visit the same adapter twice:
// `vibeporter stats` printed antigravity and windsurf rows twice and inflated
// every total, and `search` returned each hit from those agents twice while
// the duplicates ate into --limit.
func TestTargetAgentsHasNoAliasDuplicates(t *testing.T) {
	got := targetAgents("")
	if len(got) != len(canonicalAgents) {
		t.Fatalf("targetAgents() = %v (%d), want %d canonical agents", got, len(got), len(canonicalAgents))
	}
	seen := map[string]bool{}
	for _, a := range got {
		if seen[a] {
			t.Errorf("agent %q enumerated twice", a)
		}
		seen[a] = true
		if target, isAlias := agentAliases[a]; isAlias {
			t.Errorf("alias %q enumerated (canonical name is %q)", a, target)
		}
	}
	want := append([]string(nil), canonicalAgents...)
	sort.Strings(want)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("targetAgents() = %v, want %v", got, want)
		}
	}
}
