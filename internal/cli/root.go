package cli

import (
	"io"

	"github.com/spf13/cobra"
)

var codexHome string

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "codex-chat-manager",
	Short: "Manage Codex chat sessions",
	Long: `codex-chat-manager is a tool for managing OpenAI Codex chat sessions.

It provides both an interactive TUI and command-line interface for:
  - Listing sessions with filtering
  - Archiving and unarchiving sessions
  - Deleting archived sessions
  - Resuming active sessions`,
}

func init() {
	rootCmd.Version = Version
	rootCmd.PersistentFlags().StringVar(&codexHome, "codex-home", "", "override the Codex home directory")
}

func Execute(stdout, stderr io.Writer) error {
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	return rootCmd.Execute()
}
