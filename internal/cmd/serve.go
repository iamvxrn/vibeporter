package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"vibeporter/internal/web"
)

var serveAddr string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web UI (prototype) — landing + opencode-style hub",
	Long: `Start a local web server for browsing chats.

Prototype mixes landing page and opencode web hub:
  vibeporter serve                 # http://localhost:8080
  vibeporter serve --addr :3000`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := strings.TrimSpace(serveAddr)
		if addr == "" {
			addr = ":8080"
		}
		if !strings.Contains(addr, ":") {
			addr = ":" + addr
		}
		fmt.Printf("Starting vibeporter web prototype at http://localhost%s\n", addr)
		return web.Serve(addr)
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8080", "Listen address, e.g. :8080 or 127.0.0.1:3000")
	rootCmd.AddCommand(serveCmd)
}
