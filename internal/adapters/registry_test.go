package adapters

import "testing"

// CanonicalAgents is consumed by internal/cmd and internal/web as the same
// variable, not a copy each -- that identity is what makes the two surfaces
// unable to drift apart. This guards the list itself: no duplicate (which
// would make a fan-out command visit one adapter twice) and every alias
// resolves to a name that is actually on the list (a typo here would silently
// route an alias nowhere).
func TestCanonicalAgentsHasNoDuplicates(t *testing.T) {
	seen := make(map[string]bool, len(CanonicalAgents))
	for _, a := range CanonicalAgents {
		if seen[a] {
			t.Errorf("CanonicalAgents lists %q more than once", a)
		}
		seen[a] = true
	}
	if len(CanonicalAgents) == 0 {
		t.Fatal("CanonicalAgents is empty")
	}
}

func TestAgentAliasesResolveToCanonicalNames(t *testing.T) {
	canonical := make(map[string]bool, len(CanonicalAgents))
	for _, a := range CanonicalAgents {
		canonical[a] = true
	}
	for alias, target := range AgentAliases {
		if !canonical[target] {
			t.Errorf("alias %q resolves to %q, which is not in CanonicalAgents", alias, target)
		}
		if canonical[alias] {
			t.Errorf("alias %q is also listed in CanonicalAgents -- a fan-out command would visit it twice", alias)
		}
	}
}
