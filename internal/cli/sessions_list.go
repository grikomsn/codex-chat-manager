package cli

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/grikomsn/codex-chat-manager/internal/session"
	"github.com/spf13/cobra"
)

var (
	listStatusFilter    string
	listTextFilter      string
	listIncludeChildren bool
	listJSON            bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List Codex sessions",
	Long: `List all Codex sessions with optional filtering by status and text.

Examples:
  codex-chat-manager sessions list
  codex-chat-manager sessions list --status active
  codex-chat-manager sessions list --filter "my project"
  codex-chat-manager sessions list --include-children`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := resolveStore(codexHome)
		if err != nil {
			return err
		}
		snapshot, err := store.LoadSnapshot()
		if err != nil {
			return err
		}
		filtered := session.FilterGroups(snapshot.Groups, listStatusFilter, listTextFilter, listIncludeChildren)
		if listJSON {
			return printJSON(cmd.OutOrStdout(), filtered)
		}
		return printGroupTable(cmd.OutOrStdout(), filtered, listIncludeChildren)
	},
}

func init() {
	listCmd.Flags().StringVarP(&listStatusFilter, "status", "s", session.StatusFilterAll, "filter by status: all|active|archived")
	listCmd.Flags().StringVarP(&listTextFilter, "filter", "f", "", "text filter for session title, ID, or CWD")
	listCmd.Flags().BoolVar(&listIncludeChildren, "include-children", false, "include grouped child sessions in output")
	listCmd.Flags().BoolVarP(&listJSON, "json", "j", false, "render as JSON")
	sessionsCmd.AddCommand(listCmd)
}

func printGroupTable(stdout io.Writer, groups []session.SessionGroup, includeChildren bool) error {
	tw := tabwriter.NewWriter(stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tUPDATED\tPROJECT\tID\tTITLE\tCWD\tCHILDREN")
	for _, group := range groups {
		project := group.Parent.Project
		if project == "" {
			project = "unknown"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
			formatStatus(group.Status),
			group.AggregateAt.Format(time.RFC3339),
			project,
			group.Parent.ID,
			group.Parent.DisplayTitle(),
			group.Parent.Subtitle(),
			group.ChildCount,
		)
		if includeChildren {
			for _, child := range group.Children {
				childProject := child.Project
				if childProject == "" {
					childProject = "unknown"
				}
				fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\t0\n",
					formatStatus(child.Status),
					child.UpdatedAt.Format(time.RFC3339),
					childProject,
					child.ID,
					child.DisplayTitle(),
					child.Subtitle(),
				)
			}
		}
	}
	return tw.Flush()
}

func formatStatus(status session.Status) string {
	switch status {
	case session.StatusActive:
		return "● active"
	case session.StatusMixed:
		return "◍ mixed"
	case session.StatusArchived:
		return "○ archived"
	default:
		return string(status)
	}
}
