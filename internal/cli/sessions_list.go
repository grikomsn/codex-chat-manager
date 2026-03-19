package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
		cfg, err := session.ResolveConfig(codexHome)
		if err != nil {
			return err
		}
		store := session.NewStore(cfg)
		snapshot, err := store.LoadSnapshot()
		if err != nil {
			return err
		}
		filtered := filterGroups(snapshot.Groups, listStatusFilter, listTextFilter, listIncludeChildren)
		if listJSON {
			return printJSON(cmd.OutOrStdout(), filtered)
		}
		return printGroupTable(cmd.OutOrStdout(), filtered, listIncludeChildren)
	},
}

func init() {
	listCmd.Flags().StringVarP(&listStatusFilter, "status", "s", "all", "filter by status: all|active|archived")
	listCmd.Flags().StringVarP(&listTextFilter, "filter", "f", "", "text filter for session title, ID, or CWD")
	listCmd.Flags().BoolVar(&listIncludeChildren, "include-children", false, "include grouped child sessions in output")
	listCmd.Flags().BoolVarP(&listJSON, "json", "j", false, "render as JSON")
	sessionsCmd.AddCommand(listCmd)
}

func filterGroups(groups []session.SessionGroup, statusFilter, text string, includeChildren bool) []session.SessionGroup {
	text = strings.ToLower(strings.TrimSpace(text))
	statusFilter = strings.ToLower(statusFilter)
	filtered := make([]session.SessionGroup, 0, len(groups))
	for _, group := range groups {
		if statusFilter != "all" && statusFilter != "" {
			if string(group.Status) != statusFilter && !(group.MixedStatus && statusFilter == "active") && !(group.MixedStatus && statusFilter == "archived") {
				continue
			}
		}
		if text != "" && !groupMatches(group, text, includeChildren) {
			continue
		}
		filtered = append(filtered, group)
	}
	return filtered
}

func groupMatches(group session.SessionGroup, text string, includeChildren bool) bool {
	fields := []string{
		group.Parent.ID,
		group.Parent.DisplayTitle(),
		group.Parent.CWD,
		group.Parent.Source,
		group.Parent.AgentNickname,
		group.Parent.AgentRole,
	}
	if includeChildren {
		for _, child := range group.Children {
			fields = append(fields, child.ID, child.DisplayTitle(), child.CWD, child.AgentNickname, child.AgentRole)
		}
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), text) {
			return true
		}
	}
	return false
}

func printGroupTable(stdout io.Writer, groups []session.SessionGroup, includeChildren bool) error {
	tw := tabwriter.NewWriter(stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tUPDATED\tID\tTITLE\tCWD\tCHILDREN")
	for _, group := range groups {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n",
			group.Status,
			group.AggregateAt.Format(time.RFC3339),
			group.Parent.ID,
			group.Parent.DisplayTitle(),
			group.Parent.Subtitle(),
			group.ChildCount,
		)
		if includeChildren {
			for _, child := range group.Children {
				fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t0\n",
					child.Status,
					child.UpdatedAt.Format(time.RFC3339),
					child.ID,
					child.DisplayTitle(),
					child.Subtitle(),
				)
			}
		}
	}
	return tw.Flush()
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
