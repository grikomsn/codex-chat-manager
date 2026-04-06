package cli

import (
	"fmt"
	"io"

	"github.com/grikomsn/codex-chat-manager/internal/session"
	"github.com/spf13/cobra"
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

type actionRunner func(store *session.Store, ids []string) (session.ActionPlan, error)

func runActionCommand(
	cmd *cobra.Command,
	jsonOutput bool,
	ids []string,
	run actionRunner,
	failureCode func(plan session.ActionPlan) jsonErrorCode,
) error {
	store, err := resolveStore(codexHome)
	if err != nil {
		if jsonOutput {
			return printJSONCommandError(cmd, jsonErrorInventoryUnavailable, err, nil)
		}
		return err
	}

	plan, err := run(store, ids)
	if err != nil {
		if jsonOutput {
			return printJSONCommandError(cmd, failureCode(plan), err, actionPlanDetails(plan))
		}
		return err
	}
	if jsonOutput {
		return printJSON(cmd.OutOrStdout(), cmd, plan)
	}
	return printActionPlan(cmd.OutOrStdout(), plan)
}

func defaultActionFailureCode(plan session.ActionPlan) jsonErrorCode {
	return actionFailureCode(plan, jsonErrorOperationFailed)
}

func deleteActionFailureCode(plan session.ActionPlan) jsonErrorCode {
	if len(plan.BlockedByActiveIDs) > 0 {
		return jsonErrorDeleteBlockedActive
	}
	return actionFailureCode(plan, jsonErrorOperationFailed)
}
