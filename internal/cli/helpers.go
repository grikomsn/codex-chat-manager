package cli

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/grikomsn/codex-chat-manager/internal/session"
	"github.com/spf13/cobra"
)

const jsonSchemaVersion = "1"

type jsonErrorCode string

const (
	jsonErrorInvalidRequest       jsonErrorCode = "invalid_request"
	jsonErrorInventoryUnavailable jsonErrorCode = "inventory_unavailable"
	jsonErrorOperationFailed      jsonErrorCode = "operation_failed"
	jsonErrorDeleteBlockedActive  jsonErrorCode = "delete_blocked_active"
	jsonErrorSessionNotFound      jsonErrorCode = "session_not_found"
	jsonErrorResumeIneligible     jsonErrorCode = "resume_ineligible"
)

type jsonEnvelope struct {
	SchemaVersion string             `json:"schema_version"`
	Command       string             `json:"command"`
	OK            bool               `json:"ok"`
	Data          any                `json:"data,omitempty"`
	Error         *jsonEnvelopeError `json:"error,omitempty"`
}

type jsonEnvelopeError struct {
	Code    jsonErrorCode `json:"code"`
	Message string        `json:"message"`
	Details any           `json:"details,omitempty"`
}

func resolveStore(codexHome string) (*session.Store, error) {
	cfg, err := session.ResolveConfig(codexHome)
	if err != nil {
		return nil, err
	}
	return session.NewStore(cfg), nil
}

func printJSON(w io.Writer, cmd *cobra.Command, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonEnvelope{
		SchemaVersion: jsonSchemaVersion,
		Command:       jsonCommandName(cmd),
		OK:            true,
		Data:          data,
	})
}

func printJSONError(w io.Writer, cmd *cobra.Command, code jsonErrorCode, err error, details any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonEnvelope{
		SchemaVersion: jsonSchemaVersion,
		Command:       jsonCommandName(cmd),
		OK:            false,
		Error: &jsonEnvelopeError{
			Code:    code,
			Message: err.Error(),
			Details: details,
		},
	})
}

func printJSONCommandError(cmd *cobra.Command, code jsonErrorCode, err error, details any) error {
	if encodeErr := printJSONError(cmd.OutOrStdout(), cmd, code, err, details); encodeErr != nil {
		return encodeErr
	}
	return err
}

func jsonCommandName(cmd *cobra.Command) string {
	parts := []string{cmd.Name()}
	for parent := cmd.Parent(); parent != nil && parent.Parent() != nil; parent = parent.Parent() {
		parts = append([]string{parent.Name()}, parts...)
	}
	return strings.Join(parts, " ")
}

func actionPlanDetails(plan session.ActionPlan) any {
	if plan.Type == "" &&
		len(plan.RequestedIDs) == 0 &&
		len(plan.TargetIDs) == 0 &&
		len(plan.Targets) == 0 &&
		len(plan.Skipped) == 0 &&
		plan.RemovedIndexRows == 0 &&
		len(plan.RemovedSnapshots) == 0 &&
		len(plan.BlockedByActiveIDs) == 0 {
		return nil
	}
	return plan
}

func actionFailureCode(plan session.ActionPlan, operationCode jsonErrorCode) jsonErrorCode {
	if actionPlanDetails(plan) == nil {
		return jsonErrorInventoryUnavailable
	}
	return operationCode
}
