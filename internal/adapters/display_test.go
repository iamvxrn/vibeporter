package adapters

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShortPathHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got := ShortPath(home); got != "~" {
		t.Fatalf("home: got %q", got)
	}
	nested := filepath.Join(home, "extra", "git", "pkgline")
	if got := ShortPath(nested); got != "~/extra/git/pkgline" {
		t.Fatalf("nested: got %q", got)
	}
	if got := ShortPath("/tmp/somewhere"); got != "/tmp/somewhere" {
		t.Fatalf("abs: got %q", got)
	}
}

func TestClip(t *testing.T) {
	if got := Clip("  hello   \n world  ", 80); got != "hello world" {
		t.Fatalf("got %q", got)
	}
	if got := Clip("абвгд", 4); got != "абв…" {
		t.Fatalf("runes: got %q", got)
	}
}
