package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grikomsn/codex-chat-manager/internal/session"
)

func TestSessionsListJSON(t *testing.T) {
	root := t.TempDir()
	makeSessionFixture(t, root, "11111111-1111-1111-1111-111111111111", "Test title")

	stdout, _, err := executeCLI(t, root, "sessions", "list", "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.SchemaVersion != jsonSchemaVersion {
		t.Fatalf("expected schema version %q, got %q", jsonSchemaVersion, envelope.SchemaVersion)
	}
	if envelope.Command != "sessions list" {
		t.Fatalf("expected command %q, got %q", "sessions list", envelope.Command)
	}
	if !envelope.OK {
		t.Fatalf("expected ok response, got %s", stdout)
	}
	if envelope.Error != nil {
		t.Fatalf("expected no error payload, got %#v", envelope.Error)
	}
	if !strings.Contains(string(envelope.Data), `"id": "11111111-1111-1111-1111-111111111111"`) {
		t.Fatalf("expected session id in data, got %s", string(envelope.Data))
	}
}

func TestSessionsListJSONGoldenMixedGroup(t *testing.T) {
	groups := []session.SessionGroup{
		{
			Parent: session.SessionRecord{
				ID:         "11111111-1111-1111-1111-111111111111",
				Path:       "/tmp/codex/sessions/2026/03/19/rollout-parent.jsonl",
				Status:     session.StatusActive,
				CreatedAt:  mustParseTime(t, "2026-03-19T10:42:03Z"),
				UpdatedAt:  mustParseTime(t, "2026-03-19T11:00:00Z"),
				CWD:        "/tmp/project",
				Project:    "project",
				Title:      "Parent Session",
				Source:     "cli",
				ChildCount: 1,
				SizeBytes:  100,
				HasPreview: true,
			},
			Children: []session.SessionRecord{
				{
					ID:         "22222222-2222-2222-2222-222222222222",
					Path:       "/tmp/codex/archived_sessions/rollout-child.jsonl",
					Status:     session.StatusArchived,
					CreatedAt:  mustParseTime(t, "2026-03-19T10:43:03Z"),
					UpdatedAt:  mustParseTime(t, "2026-03-19T12:00:00Z"),
					CWD:        "/tmp/project",
					Project:    "project",
					Title:      "Child Session",
					Source:     "cli",
					ParentID:   "11111111-1111-1111-1111-111111111111",
					ChildCount: 0,
					SizeBytes:  50,
					HasPreview: true,
				},
			},
			Status:       session.StatusMixed,
			AggregateAt:  mustParseTime(t, "2026-03-19T12:00:00Z"),
			ChildCount:   1,
			CascadesTo:   []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"},
			ParentExists: true,
		},
	}

	var stdout bytes.Buffer
	if err := printJSON(&stdout, listCmd, groups); err != nil {
		t.Fatalf("printJSON() error = %v", err)
	}

	want, err := os.ReadFile(filepath.Join("testdata", "sessions-list-mixed.json"))
	if err != nil {
		t.Fatalf("read golden fixture: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != strings.TrimSpace(string(want)) {
		t.Fatalf("unexpected JSON fixture:\nwant:\n%s\n\ngot:\n%s", string(want), stdout.String())
	}
	if strings.Contains(stdout.String(), `"mixed_status"`) {
		t.Fatalf("unexpected mixed_status field in output: %s", stdout.String())
	}
}

func TestSessionsArchiveJSON(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeSessionFixture(t, root, id, "Test title")

	stdout, _, err := executeCLI(t, root, "sessions", "archive", "--id", id, "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertActionEnvelope(t, stdout, "sessions archive", "archive")
}

func TestSessionsUnarchiveJSON(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeArchivedSessionFixture(t, root, id)

	stdout, _, err := executeCLI(t, root, "sessions", "unarchive", "--id", id, "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertActionEnvelope(t, stdout, "sessions unarchive", "unarchive")
}

func TestSessionsDeleteJSON(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeArchivedSessionFixture(t, root, id)

	stdout, _, err := executeCLI(t, root, "sessions", "delete", "--id", id, "--yes", "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	assertActionEnvelope(t, stdout, "sessions delete", "delete")
}

func TestSessionsDeleteJSONActiveSession(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeSessionFixture(t, root, id, "Test title")

	stdout, _, err := executeCLI(t, root, "sessions", "delete", "--id", id, "--yes", "--json")
	envelope := decodeEnvelope(t, stdout)
	if err == nil {
		t.Fatal("expected delete to fail for active session")
	}
	if envelope.OK {
		t.Fatalf("expected error response, got %s", stdout)
	}
	if envelope.Error == nil {
		t.Fatal("expected error payload")
	}
	if envelope.Error.Code != string(jsonErrorDeleteBlockedActive) {
		t.Fatalf("expected error code %q, got %q", jsonErrorDeleteBlockedActive, envelope.Error.Code)
	}

	var plan testActionPlan
	if err := json.Unmarshal(envelope.Error.Details, &plan); err != nil {
		t.Fatalf("unmarshal action plan details: %v", err)
	}
	if len(plan.BlockedByActiveIDs) != 1 || plan.BlockedByActiveIDs[0] != id {
		t.Fatalf("expected blocked id %q, got %+v", id, plan.BlockedByActiveIDs)
	}
}

func TestDeleteRequiresYes(t *testing.T) {
	root := t.TempDir()
	makeArchivedSessionFixture(t, root, "11111111-1111-1111-1111-111111111111")
	_, _, err := executeCLI(t, root, "sessions", "delete", "--id", "11111111-1111-1111-1111-111111111111")
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got %v", err)
	}
}

func TestDeleteRequiresYesJSON(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	makeArchivedSessionFixture(t, root, id)

	stdout, _, err := executeCLI(t, root, "sessions", "delete", "--id", id, "--json")
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got %v", err)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.OK {
		t.Fatalf("expected error response, got %s", stdout)
	}
	if len(envelope.Data) != 0 && string(envelope.Data) != "null" {
		t.Fatalf("expected no top-level data payload, got %s", string(envelope.Data))
	}
	if envelope.Error == nil {
		t.Fatalf("expected error payload, got nil")
	}
	if envelope.Error.Code != string(jsonErrorInvalidRequest) {
		t.Fatalf("expected error code %q, got %q", jsonErrorInvalidRequest, envelope.Error.Code)
	}
	if !strings.Contains(envelope.Error.Message, "--yes") {
		t.Fatalf("expected --yes error message, got %#v", envelope.Error)
	}
	var details map[string]string
	if err := json.Unmarshal(envelope.Error.Details, &details); err != nil {
		t.Fatalf("unmarshal invalid request details: %v", err)
	}
	if details["required_flag"] != "yes" {
		t.Fatalf("expected required_flag=yes, got %+v", details)
	}
}

func TestSessionsListJSONError(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "session_index.jsonl"), 0o755); err != nil {
		t.Fatalf("mkdir session index path: %v", err)
	}

	stdout, _, err := executeCLI(t, root, "sessions", "list", "--json")
	if err == nil {
		t.Fatalf("expected list error, got nil")
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.OK {
		t.Fatalf("expected error response, got %s", stdout)
	}
	if len(envelope.Data) != 0 && string(envelope.Data) != "null" {
		t.Fatalf("expected no top-level data payload, got %s", string(envelope.Data))
	}
	if envelope.Error == nil {
		t.Fatalf("expected error payload, got nil")
	}
	if envelope.Error.Code != string(jsonErrorInventoryUnavailable) {
		t.Fatalf("expected error code %q, got %q", jsonErrorInventoryUnavailable, envelope.Error.Code)
	}
}

func TestSessionsResumeJSON(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	cwd := filepath.Join(root, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	makeSessionFixtureWithCWD(t, root, id, "Test title", cwd)

	stdout, _, err := executeCLI(t, root, "sessions", "resume", "--id", id, "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected ok response, got %s", stdout)
	}
	var result testResumeResult
	if err := json.Unmarshal(envelope.Data, &result); err != nil {
		t.Fatalf("unmarshal resume result: %v", err)
	}
	if result.Executed {
		t.Fatal("expected non-executing JSON resume response")
	}
	if result.Intent.SessionID != id {
		t.Fatalf("expected session id %q, got %q", id, result.Intent.SessionID)
	}
	if !result.Intent.Eligible {
		t.Fatal("expected eligible intent")
	}
	if result.Intent.Status != string(session.StatusActive) {
		t.Fatalf("expected active status, got %q", result.Intent.Status)
	}
	if result.Intent.WorkingDirectory != cwd {
		t.Fatalf("expected working directory %q, got %q", cwd, result.Intent.WorkingDirectory)
	}
	if result.Intent.Executable != "codex" {
		t.Fatalf("expected codex executable, got %q", result.Intent.Executable)
	}
	if len(result.Intent.Args) != 2 || result.Intent.Args[1] != id {
		t.Fatalf("unexpected args: %+v", result.Intent.Args)
	}
	if result.Intent.EnvOverrides[session.EnvCodexHome] != root {
		t.Fatalf("expected CODEX_HOME override %q, got %q", root, result.Intent.EnvOverrides[session.EnvCodexHome])
	}
}

func TestSessionsResumeJSONArchivedError(t *testing.T) {
	root := t.TempDir()
	id := "11111111-1111-1111-1111-111111111111"
	cwd := filepath.Join(root, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("mkdir cwd: %v", err)
	}
	makeArchivedSessionFixtureWithCWD(t, root, id, cwd)

	stdout, _, err := executeCLI(t, root, "sessions", "resume", "--id", id, "--json")
	if err == nil {
		t.Fatal("expected resume error for archived session")
	}

	envelope := decodeEnvelope(t, stdout)
	if envelope.OK {
		t.Fatalf("expected error response, got %s", stdout)
	}
	if envelope.Error == nil {
		t.Fatal("expected error payload")
	}
	if envelope.Error.Code != string(jsonErrorResumeIneligible) {
		t.Fatalf("expected error code %q, got %q", jsonErrorResumeIneligible, envelope.Error.Code)
	}
	var result testResumeResult
	if err := json.Unmarshal(envelope.Error.Details, &result); err != nil {
		t.Fatalf("unmarshal resume error details: %v", err)
	}
	if result.Intent.SessionID != id {
		t.Fatalf("expected session id %q, got %q", id, result.Intent.SessionID)
	}
	if result.Intent.Eligible {
		t.Fatal("expected ineligible intent")
	}
	if result.Intent.Status != string(session.StatusArchived) {
		t.Fatalf("expected archived status, got %q", result.Intent.Status)
	}
}

type testEnvelope struct {
	SchemaVersion string          `json:"schema_version"`
	Command       string          `json:"command"`
	OK            bool            `json:"ok"`
	Data          json.RawMessage `json:"data"`
	Error         *testError      `json:"error"`
}

type testError struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details"`
}

type testActionPlan struct {
	Type               string   `json:"type"`
	BlockedByActiveIDs []string `json:"blocked_by_active_ids"`
}

type testResumeResult struct {
	Intent   testResumeIntent `json:"intent"`
	Executed bool             `json:"executed"`
}

type testResumeIntent struct {
	RequestedID      string            `json:"requested_id"`
	SessionID        string            `json:"session_id"`
	Status           string            `json:"status"`
	Eligible         bool              `json:"eligible"`
	WorkingDirectory string            `json:"working_directory"`
	Executable       string            `json:"executable"`
	Args             []string          `json:"args"`
	EnvOverrides     map[string]string `json:"env_overrides"`
}

func executeCLI(t *testing.T, root string, args ...string) (string, string, error) {
	t.Helper()

	resetCLIFlags()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(append(args, "--codex-home", root))

	err := rootCmd.Execute()

	return stdout.String(), stderr.String(), err
}

func resetCLIFlags() {
	listStatusFilter = "all"
	listTextFilter = ""
	listIncludeChildren = false
	listJSON = false

	archiveIDs = nil
	archiveJSON = false

	unarchiveIDs = nil
	unarchiveJSON = false

	deleteIDs = nil
	deleteJSON = false
	deleteYes = false

	resumeID = ""
	resumeJSON = false
	resumePrintCmd = false
	resumeExecute = false
}

func decodeEnvelope(t *testing.T, stdout string) testEnvelope {
	t.Helper()

	var envelope testEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v\nstdout=%s", err, stdout)
	}
	return envelope
}

func assertActionEnvelope(t *testing.T, stdout, wantCommand, wantType string) {
	t.Helper()

	envelope := decodeEnvelope(t, stdout)
	if envelope.SchemaVersion != jsonSchemaVersion {
		t.Fatalf("expected schema version %q, got %q", jsonSchemaVersion, envelope.SchemaVersion)
	}
	if envelope.Command != wantCommand {
		t.Fatalf("expected command %q, got %q", wantCommand, envelope.Command)
	}
	if !envelope.OK {
		t.Fatalf("expected ok response, got %s", stdout)
	}
	if envelope.Error != nil {
		t.Fatalf("expected no error payload, got %#v", envelope.Error)
	}
	var plan testActionPlan
	if err := json.Unmarshal(envelope.Data, &plan); err != nil {
		t.Fatalf("unmarshal action plan: %v", err)
	}
	if plan.Type != wantType {
		t.Fatalf("expected action type %q, got %q", wantType, plan.Type)
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}

func makeSessionFixture(t *testing.T, root, id, title string) {
	t.Helper()
	path := filepath.Join(root, "sessions", "2026", "03", "19")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session-meta","payload":{"id":"` + id + `","cwd":"/tmp/app","source":"vscode"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"` + title + `"}}` + "\n"
	if err := os.WriteFile(filepath.Join(path, "rollout-2026-03-19T10-42-03-"+id+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeSessionFixtureWithCWD(t *testing.T, root, id, title, cwd string) {
	t.Helper()
	path := filepath.Join(root, "sessions", "2026", "03", "19")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session-meta","payload":{"id":"` + id + `","cwd":"` + cwd + `","source":"vscode"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"` + title + `"}}` + "\n"
	if err := os.WriteFile(filepath.Join(path, "rollout-2026-03-19T10-42-03-"+id+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeArchivedSessionFixture(t *testing.T, root, id string) {
	t.Helper()
	path := filepath.Join(root, "archived_sessions")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session-meta","payload":{"id":"` + id + `","cwd":"/tmp/app","source":"vscode"}}` + "\n"
	if err := os.WriteFile(filepath.Join(path, "rollout-2026-03-19T10-42-03-"+id+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeArchivedSessionFixtureWithCWD(t *testing.T, root, id, cwd string) {
	t.Helper()
	path := filepath.Join(root, "archived_sessions")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session-meta","payload":{"id":"` + id + `","cwd":"` + cwd + `","source":"vscode"}}` + "\n"
	if err := os.WriteFile(filepath.Join(path, "rollout-2026-03-19T10-42-03-"+id+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
