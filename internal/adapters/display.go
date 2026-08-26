package adapters

import (
	"os"
	"path/filepath"
	"strings"
)

// ShortPath renders p relative to the home directory when possible (~/…).
func ShortPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.Clean(p)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	home = filepath.Clean(home)
	if p == home {
		return "~"
	}
	rel, err := filepath.Rel(home, p)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}

// Clip collapses whitespace and truncates s to at most n runes.
func Clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if n <= 0 || len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}
