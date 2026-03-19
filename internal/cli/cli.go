package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/grikomsn/codex-chat-manager/internal/session"
	"github.com/grikomsn/codex-chat-manager/internal/tui"
)

// Run executes the root CLI.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return runTUI(stdout, stderr, "")
	}
	switch args[0] {
	case "tui":
		fs := flag.NewFlagSet("tui", flag.ContinueOnError)
		fs.SetOutput(stderr)
		codexHome := fs.String("codex-home", "", "override the Codex home directory")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		return runTUI(stdout, stderr, *codexHome)
	case "sessions":
		return runSessions(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "codex-chat-manager <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  tui")
	fmt.Fprintln(w, "  sessions list")
	fmt.Fprintln(w, "  sessions archive --id ID...")
	fmt.Fprintln(w, "  sessions unarchive --id ID...")
	fmt.Fprintln(w, "  sessions delete --id ID... [--yes]")
	fmt.Fprintln(w, "  sessions resume --id ID")
}

func runSessions(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing sessions subcommand")
	}
	switch args[0] {
	case "list":
		return runList(args[1:], stdout, stderr)
	case "archive":
		return runAction(args[1:], stdout, stderr, session.ActionArchive)
	case "unarchive":
		return runAction(args[1:], stdout, stderr, session.ActionUnarchive)
	case "delete":
		return runDelete(args[1:], stdout, stderr)
	case "resume":
		return runResume(ctx, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown sessions subcommand %q", args[0])
	}
}

func runTUI(stdout, stderr io.Writer, override string) error {
	cfg, err := session.ResolveConfig(override)
	if err != nil {
		return err
	}
	return tui.Run(cfg, stdout, stderr)
}

func runList(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	statusFlag := fs.String("status", "all", "filter by status: all|active|archived")
	filter := fs.String("filter", "", "text filter")
	includeChildren := fs.Bool("include-children", false, "include grouped child sessions in output")
	jsonOut := fs.Bool("json", false, "render as JSON")
	codexHome := fs.String("codex-home", "", "override the Codex home directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := session.ResolveConfig(*codexHome)
	if err != nil {
		return err
	}
	store := session.NewStore(cfg)
	snapshot, err := store.LoadSnapshot()
	if err != nil {
		return err
	}
	filtered := filterGroups(snapshot.Groups, *statusFlag, *filter, *includeChildren)
	if *jsonOut {
		return printJSON(stdout, filtered)
	}
	return printGroupTable(stdout, filtered, *includeChildren)
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

func runAction(args []string, stdout, stderr io.Writer, action session.ActionType) error {
	fs := flag.NewFlagSet(string(action), flag.ContinueOnError)
	fs.SetOutput(stderr)
	var ids multiValue
	fs.Var(&ids, "id", "session id")
	jsonOut := fs.Bool("json", false, "render as JSON")
	codexHome := fs.String("codex-home", "", "override the Codex home directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("%s requires at least one --id", action)
	}
	cfg, err := session.ResolveConfig(*codexHome)
	if err != nil {
		return err
	}
	store := session.NewStore(cfg)
	var plan session.ActionPlan
	switch action {
	case session.ActionArchive:
		plan, err = store.Archive(ids)
	case session.ActionUnarchive:
		plan, err = store.Unarchive(ids)
	default:
		return fmt.Errorf("unsupported action %s", action)
	}
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(stdout, plan)
	}
	return printActionPlan(stdout, plan)
}

func runDelete(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var ids multiValue
	fs.Var(&ids, "id", "session id")
	jsonOut := fs.Bool("json", false, "render as JSON")
	yes := fs.Bool("yes", false, "skip interactive confirmation")
	codexHome := fs.String("codex-home", "", "override the Codex home directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("delete requires at least one --id")
	}
	if !*yes {
		return fmt.Errorf("delete requires --yes in non-interactive mode")
	}
	cfg, err := session.ResolveConfig(*codexHome)
	if err != nil {
		return err
	}
	store := session.NewStore(cfg)
	plan, err := store.Delete(ids)
	if err != nil {
		if *jsonOut {
			_ = printJSON(stdout, plan)
		}
		return err
	}
	if *jsonOut {
		return printJSON(stdout, plan)
	}
	return printActionPlan(stdout, plan)
}

func runResume(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	_ = stdout
	fs := flag.NewFlagSet("resume", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "session id")
	codexHome := fs.String("codex-home", "", "override the Codex home directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("resume requires --id")
	}
	cfg, err := session.ResolveConfig(*codexHome)
	if err != nil {
		return err
	}
	store := session.NewStore(cfg)
	return store.Resume(ctx, *id)
}

func printActionPlan(w io.Writer, plan session.ActionPlan) error {
	fmt.Fprintf(w, "%s: %d changed", plan.Type, len(plan.Targets))
	if plan.RemovedIndexRows > 0 {
		fmt.Fprintf(w, ", %d index rows removed", plan.RemovedIndexRows)
	}
	if len(plan.RemovedSnapshots) > 0 {
		fmt.Fprintf(w, ", %d snapshots removed", len(plan.RemovedSnapshots))
	}
	fmt.Fprintln(w)
	for _, target := range plan.Targets {
		fmt.Fprintf(w, "- %s %s\n", target.ID, target.Path)
	}
	for _, skip := range plan.Skipped {
		fmt.Fprintf(w, "- skipped %s: %s\n", skip.ID, skip.Reason)
	}
	return nil
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type multiValue []string

func (m *multiValue) String() string {
	return strings.Join(*m, ",")
}

func (m *multiValue) Set(v string) error {
	*m = append(*m, v)
	return nil
}
