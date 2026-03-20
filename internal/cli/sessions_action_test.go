package cli

import (
	"encoding/json"
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

func TestSessionsDeleteJSONActiveSessionSchema(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeSessionFixture(t, root, id, "Active Session")

	stdout, _, err := executeCLI(t, root, "sessions", "delete", "--id", id, "--yes", "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected ok response, got %s", stdout)
	}

	var details struct {
		Type               string   `json:"type"`
		RequestedIDs       []string `json:"requested_ids"`
		BlockedByActiveIDs []string `json:"blocked_by_active_ids"`
		Targets            []any    `json:"targets"`
		Skipped            []any    `json:"skipped"`
	}
	if err := json.Unmarshal(envelope.Data, &details); err != nil {
		t.Fatalf("unmarshal action details: %v", err)
	}
	if details.Type != "delete" {
		t.Fatalf("expected delete details, got %+v", details)
	}
	if len(details.RequestedIDs) != 1 || details.RequestedIDs[0] != id {
		t.Fatalf("unexpected requested ids: %+v", details.RequestedIDs)
	}
	if len(details.BlockedByActiveIDs) != 0 {
		t.Fatalf("expected no blocked ids, got %+v", details.BlockedByActiveIDs)
	}
}
