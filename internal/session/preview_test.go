package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderPreviewWrapsBlocks(t *testing.T) {
	t.Parallel()
	doc := PreviewDocument{
		SessionID: "one",
		Title:     "Title",
		Blocks: []PreviewBlock{
			{Kind: PreviewUser, Title: "User", Body: "this is a long line that should wrap into multiple pieces for the viewport"},
			{Kind: PreviewToolCall, Title: "exec_command", Body: "{\"cmd\":\"ls -la\"}"},
		},
	}
	got := RenderPreview(doc, 20, false, nil)
	if got == "" {
		t.Fatal("expected rendered preview")
	}
	if !containsAll(got, []string{"Title", "User", "> ls -la"}) {
		t.Fatalf("unexpected preview output %q", got)
	}
}

func TestRenderPreviewHidesSystemContextByDefault(t *testing.T) {
	t.Parallel()
	doc := PreviewDocument{
		SessionID: "one",
		Title:     "Title",
		Blocks: []PreviewBlock{
			{Kind: PreviewEvent, Title: "Context", Body: "system instructions"},
			{Kind: PreviewAssistant, Title: "Assistant", Body: "hello"},
		},
	}
	hidden := RenderPreview(doc, 80, false, nil)
	if strings.Contains(hidden, "system instructions") {
		t.Fatalf("expected system context to be hidden, got %q", hidden)
	}
	shown := RenderPreview(doc, 80, true, nil)
	if !strings.Contains(shown, "system instructions") {
		t.Fatalf("expected system context to be shown, got %q", shown)
	}
}

func containsAll(s string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

func TestInjectedAgentsContextIsHiddenByDefault(t *testing.T) {
	t.Parallel()
	raw := jsonRaw(`{"type":"user_message","message":"# AGENTS.md instructions for /tmp/repo\n\n<INSTRUCTIONS>\nsecret\n</INSTRUCTIONS>\n<environment_context>\n...</environment_context>"}`)
	block := eventBlock(raw)
	if block == nil {
		t.Fatal("expected block")
	}
	if block.Title != "Context" {
		t.Fatalf("expected Context title, got %q", block.Title)
	}
	if block.Kind != PreviewEvent {
		t.Fatalf("expected event kind, got %q", block.Kind)
	}

	doc := PreviewDocument{
		SessionID: "one",
		Title:     "Title",
		Blocks: []PreviewBlock{
			*block,
			{Kind: PreviewAssistant, Title: "Assistant", Body: "hello"},
		},
	}
	hidden := RenderPreview(doc, 80, false, nil)
	if strings.Contains(hidden, "AGENTS.md instructions") {
		t.Fatalf("expected injected context to be hidden, got %q", hidden)
	}
	shown := RenderPreview(doc, 80, true, nil)
	if !strings.Contains(shown, "AGENTS.md instructions") {
		t.Fatalf("expected injected context to be shown, got %q", shown)
	}
}

func TestParsePreviewPairsToolCallOutputsByCallID(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "rollout.jsonl")
	body := strings.Join([]string{
		`{"type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call-1","arguments":{"cmd":"ls -la"}}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"done"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := parsePreview(SessionRecord{ID: "one", Path: path, Title: "Title"})
	if err != nil {
		t.Fatalf("parsePreview() error = %v", err)
	}
	if len(doc.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(doc.Blocks))
	}
	if got := doc.Blocks[0].Title; got != "exec_command (call-1)" {
		t.Fatalf("unexpected tool call title %q", got)
	}
	if got := doc.Blocks[1].Title; got != "exec_command (call-1)" {
		t.Fatalf("unexpected tool output title %q", got)
	}
}

func jsonRaw(value string) []byte {
	return []byte(value)
}

func TestRenderPreviewHidesFirstAssistantWhenItLooksLikeInitialContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "rollout.jsonl")
	body := strings.Join([]string{
		`{"type":"event_msg","payload":{"type":"user_message","message":"hi"}}`,
		`{"type":"event_msg","payload":{"type":"agent_message","message":"# AGENTS.md instructions for /tmp/repo\n\n<INSTRUCTIONS>\nsecret\n</INSTRUCTIONS>\n<environment_context>\n...</environment_context>"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"real assistant message"}]}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := parsePreview(SessionRecord{ID: "one", Path: path, Title: "Title"})
	if err != nil {
		t.Fatalf("parsePreview() error = %v", err)
	}

	hidden := RenderPreview(doc, 80, false, nil)
	if strings.Contains(hidden, "secret") {
		t.Fatalf("expected injected assistant context to be hidden, got %q", hidden)
	}
	if !strings.Contains(hidden, "real assistant message") {
		t.Fatalf("expected assistant message to remain visible, got %q", hidden)
	}
}

func TestParsePreviewDedupesUserMessagesAcrossEnvelopes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "rollout.jsonl")
	body := strings.Join([]string{
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"hello"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := parsePreview(SessionRecord{ID: "one", Path: path, Title: "Title"})
	if err != nil {
		t.Fatalf("parsePreview() error = %v", err)
	}
	count := 0
	for _, block := range doc.Blocks {
		if block.Kind == PreviewUser && strings.TrimSpace(block.Body) == "hello" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 deduped user message, got %d (%#v)", count, doc.Blocks)
	}
}

func TestParsePreviewDedupesAssistantMessagesAcrossEnvelopes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "rollout.jsonl")
	body := strings.Join([]string{
		`{"type":"event_msg","payload":{"type":"agent_message","message":"hello"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := parsePreview(SessionRecord{ID: "one", Path: path, Title: "Title"})
	if err != nil {
		t.Fatalf("parsePreview() error = %v", err)
	}
	count := 0
	for _, block := range doc.Blocks {
		if block.Kind == PreviewAssistant && strings.TrimSpace(block.Body) == "hello" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 deduped assistant message, got %d (%#v)", count, doc.Blocks)
	}
}

func TestParsePreviewRendersCustomToolCalls(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "rollout.jsonl")
	body := strings.Join([]string{
		`{"type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch","call_id":"call-1","status":"completed","input":"*** Begin Patch\n*** End Patch\n"}}`,
		`{"type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"call-1","output":"{\"output\":\"Success. Updated the following files:\\nM a.txt\\n\",\"metadata\":{\"exit_code\":0}}"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	doc, err := parsePreview(SessionRecord{ID: "one", Path: path, Title: "Title"})
	if err != nil {
		t.Fatalf("parsePreview() error = %v", err)
	}
	if len(doc.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(doc.Blocks))
	}
	if got := doc.Blocks[0].Title; got != "apply_patch (call-1)" {
		t.Fatalf("unexpected tool call title %q", got)
	}
	if !strings.Contains(doc.Blocks[0].Body, "*** Begin Patch") {
		t.Fatalf("expected apply_patch input body, got %q", doc.Blocks[0].Body)
	}
	if got := doc.Blocks[1].Title; got != "apply_patch (call-1)" {
		t.Fatalf("unexpected tool output title %q", got)
	}
	if !strings.Contains(doc.Blocks[1].Body, "Success. Updated") {
		t.Fatalf("expected normalized tool output, got %q", doc.Blocks[1].Body)
	}
}
