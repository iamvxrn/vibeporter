package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var portDir string

var portCmd = &cobra.Command{
	Use:   "port-config",
	Short: "Port project configuration files (like CLAUDE.md) between agents",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Porting configs in %s from %s to %s...\n", portDir, fromAgent, toAgent)

		if fromAgent == "claudecode" && toAgent == "gemini" {
			portFile(portDir, ".claudeignore", ".geminiignore")
			portFile(portDir, "CLAUDE.md", "GEMINI.md")
		} else if fromAgent == "cursor" && toAgent == "gemini" {
			portFile(portDir, ".cursorignore", ".geminiignore")
			portFile(portDir, ".cursorrules", "GEMINI.md")
		} else if fromAgent == "opencode" && toAgent == "gemini" {
			portFile(portDir, ".opencodeignore", ".geminiignore")
			portFile(portDir, "OPENCODE.md", "GEMINI.md")
		} else if (fromAgent == "kimicode" || fromAgent == "kimi") && toAgent == "gemini" {
			portFile(portDir, "AGENTS.md", "GEMINI.md")
		} else {
			fmt.Printf("Config porting from %s to %s is not supported yet.\n", fromAgent, toAgent)
		}
	},
}

func portFile(dir, source, target string) {
	srcPath := filepath.Join(dir, source)
	tgtPath := filepath.Join(dir, target)

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return // Source file doesn't exist, nothing to do
	}

	if _, err := os.Stat(tgtPath); err == nil {
		fmt.Printf("Warning: Target %s already exists, skipping %s.\n", tgtPath, source)
		return
	}

	err := copyFile(srcPath, tgtPath)
	if err != nil {
		fmt.Printf("Error porting %s to %s: %v\n", source, target, err)
		return
	}
	fmt.Printf("Successfully ported %s -> %s\n", source, target)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

func init() {
	portCmd.Flags().StringVar(&fromAgent, "from", "", "Source agent (e.g. claudecode, cursor)")
	portCmd.Flags().StringVar(&toAgent, "to", "", "Target agent (e.g. gemini)")
	portCmd.Flags().StringVar(&portDir, "dir", ".", "Directory to run in")

	_ = portCmd.MarkFlagRequired("from")
	_ = portCmd.MarkFlagRequired("to")

	rootCmd.AddCommand(portCmd)
}
