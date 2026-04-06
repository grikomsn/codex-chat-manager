package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSessionsArchiveJSONSkipped(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeArchivedSessionFixture(t, root, id)

	stdout, _, err := executeCLI(t, root, "sessions", "archive", "--id", id, "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected ok response, got %s", stdout)
	}

	var plan struct {
		Type    string `json:"type"`
		Targets []any  `json:"targets"`
		Skipped []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"skipped"`
	}
	if err := json.Unmarshal(envelope.Data, &plan); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if plan.Type != "archive" {
		t.Fatalf("expected archive plan, got %+v", plan)
	}
	if len(plan.Targets) != 0 {
		t.Fatalf("expected no targets, got %+v", plan.Targets)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].ID != id || plan.Skipped[0].Reason != "already archived" {
		t.Fatalf("unexpected skipped entries: %+v", plan.Skipped)
	}
}

func TestSessionsUnarchiveJSONSkipped(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeSessionFixture(t, root, id, "Active Session")

	stdout, _, err := executeCLI(t, root, "sessions", "unarchive", "--id", id, "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected ok response, got %s", stdout)
	}

	var plan struct {
		Type    string `json:"type"`
		Targets []any  `json:"targets"`
		Skipped []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"skipped"`
	}
	if err := json.Unmarshal(envelope.Data, &plan); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if plan.Type != "unarchive" {
		t.Fatalf("expected unarchive plan, got %+v", plan)
	}
	if len(plan.Targets) != 0 {
		t.Fatalf("expected no targets, got %+v", plan.Targets)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].ID != id || plan.Skipped[0].Reason != "already active" {
		t.Fatalf("unexpected skipped entries: %+v", plan.Skipped)
	}
}

func TestSessionsDeleteJSONBlockedActiveSessionSchema(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeSessionFixture(t, root, id, "Active Session")

	stdout, _, err := executeCLI(t, root, "sessions", "delete", "--id", id, "--yes", "--json")
	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		// expected
	} else {
		t.Fatalf("expected error response, got %s", stdout)
	}
	if err == nil {
		t.Fatal("expected delete to fail for active session")
	}
	if envelope.Error == nil {
		t.Fatal("expected error payload")
	}
	if envelope.Error.Code != string(jsonErrorDeleteBlockedActive) {
		t.Fatalf("expected error code %q, got %q", jsonErrorDeleteBlockedActive, envelope.Error.Code)
	}

	var details struct {
		Type               string   `json:"type"`
		RequestedIDs       []string `json:"requested_ids"`
		BlockedByActiveIDs []string `json:"blocked_by_active_ids"`
		Targets            []any    `json:"targets"`
		Skipped            []any    `json:"skipped"`
	}
	if err := json.Unmarshal(envelope.Error.Details, &details); err != nil {
		t.Fatalf("unmarshal action details: %v", err)
	}
	if details.Type != "delete" {
		t.Fatalf("expected delete details, got %+v", details)
	}
	if len(details.RequestedIDs) != 1 || details.RequestedIDs[0] != id {
		t.Fatalf("unexpected requested ids: %+v", details.RequestedIDs)
	}
	if len(details.BlockedByActiveIDs) != 1 || details.BlockedByActiveIDs[0] != id {
		t.Fatalf("expected blocked ids [%q], got %+v", id, details.BlockedByActiveIDs)
	}
	if !strings.Contains(envelope.Error.Message, id) {
		t.Fatalf("expected error message to mention blocked id, got %#v", envelope.Error)
	}
}

func TestSessionsDeleteJSONSkipsUnknownIDs(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeArchivedSessionFixture(t, root, id)

	stdout, _, err := executeCLI(t, root, "sessions", "delete", "--id", id, "--id", "missing", "--yes", "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected ok response, got %s", stdout)
	}

	var plan struct {
		Type    string `json:"type"`
		Skipped []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		} `json:"skipped"`
	}
	if err := json.Unmarshal(envelope.Data, &plan); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	if plan.Type != "delete" {
		t.Fatalf("expected delete plan, got %+v", plan)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].ID != "missing" || plan.Skipped[0].Reason != "not found" {
		t.Fatalf("unexpected skipped entries: %+v", plan.Skipped)
	}
}
