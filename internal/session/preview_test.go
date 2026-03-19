package session

import (
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
	if !containsAll(got, []string{"Title", "[User]", "[exec_command]"}) {
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
