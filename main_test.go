package main

import (
	"os"
	"testing"
)

func TestMainHelp(t *testing.T) {
	old := os.Args
	os.Args = []string{"vibeporter", "--help"}
	t.Cleanup(func() { os.Args = old })
	main()
}
