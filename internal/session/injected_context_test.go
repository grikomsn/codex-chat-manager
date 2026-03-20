package session

import "testing"

func TestIsInjectedAgentsContextHeuristic(t *testing.T) {
	t.Parallel()
	if !isInjectedAgentsContext("# AGENTS.md instructions for /tmp/repo\n\n<INSTRUCTIONS>\nsecret\n</INSTRUCTIONS>\n<environment_context>\n...</environment_context>") {
		t.Fatal("expected injected context to match")
	}
	if isInjectedAgentsContext("Here is a plan.\n\nIt mentions AGENTS.md instructions for /tmp/repo.\n\n<INSTRUCTIONS>\nsecret\n</INSTRUCTIONS>\n<environment_context>\n...</environment_context>") {
		t.Fatal("expected non-injected message to not match")
	}
}
