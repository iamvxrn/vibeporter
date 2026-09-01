package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "vibeporter",
	Short:         "Migrate chat histories and configs between AI coding agents",
	Version:       "0.5.0",
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if handoffJSON {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
			os.Exit(1)
		}
		fmt.Println(err)
		os.Exit(1)
	}
}
