package cli

import (
	"fmt"
	"io"

	"github.com/grikomsn/codex-chat-manager/internal/session"
)

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
