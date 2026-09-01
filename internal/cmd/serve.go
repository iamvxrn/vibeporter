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
	Short: "Start the local-only web UI",
	Long: `Start a local web server for browsing chats.

  vibeporter serve                         # http://127.0.0.1:8080
  vibeporter serve --addr 127.0.0.1:3000
  vibeporter serve --addr 0.0.0.0:8080     # exposes chat data to the network`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := strings.TrimSpace(serveAddr)
		if addr == "" {
			addr = "127.0.0.1:8080"
		}
		if strings.HasPrefix(addr, ":") {
			addr = "127.0.0.1" + addr
		} else if !strings.Contains(addr, ":") {
			addr = "127.0.0.1:" + addr
		}
		if !strings.HasPrefix(addr, "127.0.0.1:") && !strings.HasPrefix(addr, "localhost:") {
			fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: serving chat data on %s. Anyone who can reach this address can access the local API.\n", addr)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Starting local-only vibeporter web at http://%s\n", addr)
		return web.Serve(addr)
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", "127.0.0.1:8080", "Listen address; external addresses expose chat data")
	rootCmd.AddCommand(serveCmd)
}
