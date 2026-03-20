package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/grikomsn/codex-chat-manager/internal/session"
	"github.com/spf13/cobra"
)

var (
	resumeID       string
	resumeJSON     bool
	resumePrintCmd bool
	resumeExecute  bool
)

var resumeCmd = &cobra.Command{
	Use:   "resume --id ID",
	Short: "Resume a Codex session",
	Long: `Resume an active Codex session in a new terminal.

Only active (non-archived) sessions can be resumed.

Examples:
  codex-chat-manager sessions resume --id abc123
  codex-chat-manager sessions resume --id abc123 --json
  codex-chat-manager sessions resume --id abc123 --print-cmd`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if resumePrintCmd && resumeExecute {
			err := fmt.Errorf("resume does not allow --print-cmd with --execute")
			if resumeJSON {
				return printJSONCommandError(cmd, jsonErrorInvalidRequest, err, map[string]bool{
					"print_cmd": true,
					"execute":   true,
				})
			}
			return err
		}

		store, err := resolveStore(codexHome)
		if err != nil {
			if resumeJSON {
				return printJSONCommandError(cmd, jsonErrorInventoryUnavailable, err, nil)
			}
			return err
		}

		intent, err := store.ResumeIntent(resumeID)
		result := resumeJSONResult{Intent: intent}
		if err != nil {
			if resumeJSON {
				return printJSONCommandError(cmd, resumeErrorCode(err), err, resumeResultDetails(result))
			}
			return err
		}

		if resumeJSON {
			if resumeExecute {
				if err := store.Resume(nil, resumeID); err != nil {
					return printJSONCommandError(cmd, jsonErrorOperationFailed, err, resumeResultDetails(result))
				}
				result.Executed = true
			}
			return printJSON(cmd.OutOrStdout(), cmd, result)
		}
		if resumePrintCmd {
			return printResumeCommand(cmd.OutOrStdout(), intent)
		}
		return store.Resume(nil, resumeID)
	},
}

func init() {
	resumeCmd.Flags().StringVarP(&resumeID, "id", "i", "", "session ID to resume (required)")
	resumeCmd.Flags().BoolVarP(&resumeJSON, "json", "j", false, "render as JSON without executing unless --execute is set")
	resumeCmd.Flags().BoolVar(&resumePrintCmd, "print-cmd", false, "print the resolved codex resume command without executing it")
	resumeCmd.Flags().BoolVar(&resumeExecute, "execute", false, "execute the resume command even in JSON mode")
	resumeCmd.MarkFlagRequired("id")
	sessionsCmd.AddCommand(resumeCmd)
}

type resumeJSONResult struct {
	Intent   session.ResumeIntent `json:"intent"`
	Executed bool                 `json:"executed"`
}

func resumeErrorCode(err error) jsonErrorCode {
	switch {
	case errors.Is(err, session.ErrSessionNotFound):
		return jsonErrorSessionNotFound
	case errors.Is(err, session.ErrResumeIneligible):
		return jsonErrorResumeIneligible
	default:
		return jsonErrorOperationFailed
	}
}

func resumeResultDetails(result resumeJSONResult) any {
	if result.Intent.RequestedID == "" && !result.Executed {
		return nil
	}
	return result
}

func printResumeCommand(w io.Writer, intent session.ResumeIntent) error {
	fmt.Fprintln(w, formatResumeCommand(intent))
	return nil
}

func formatResumeCommand(intent session.ResumeIntent) string {
	parts := []string{"cd", shellQuote(intent.WorkingDirectory), "&&"}

	keys := make([]string, 0, len(intent.EnvOverrides))
	for key := range intent.EnvOverrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+"="+shellQuote(intent.EnvOverrides[key]))
	}

	parts = append(parts, shellQuote(intent.Executable))
	for _, arg := range intent.Args {
		parts = append(parts, shellQuote(arg))
	}

	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"`$&|;<>(){}[]*?!#~") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
