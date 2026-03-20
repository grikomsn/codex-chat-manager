package session

import (
	"strings"
	"testing"
)

func TestMarkdownRendererRender(t *testing.T) {
	t.Parallel()
	mr := NewMarkdownRenderer()

	content := "# Hello\n\nThis is **bold** and *italic*."
	rendered, err := mr.Render(content, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rendered == "" {
		t.Fatal("expected rendered content")
	}
	// Check that markdown was converted (should have ANSI codes)
	if rendered == content {
		t.Fatal("expected markdown to be rendered, not returned as-is")
	}
}

func TestMarkdownRendererCaching(t *testing.T) {
	t.Parallel()
	mr := NewMarkdownRenderer()

	content := "# Test"
	// First render at width 80
	_, err := mr.Render(content, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second render at same width should use cache
	_, err = mr.Render(content, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Render at different width
	_, err = mr.Render(content, 40)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMarkdownRendererClearCache(t *testing.T) {
	t.Parallel()
	mr := NewMarkdownRenderer()

	content := "# Test"
	_, err := mr.Render(content, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Clear cache
	mr.ClearCache()

	// Render again - should work fine
	_, err = mr.Render(content, 80)
	if err != nil {
		t.Fatalf("unexpected error after cache clear: %v", err)
	}
}

func TestMarkdownRendererCodeHighlighting(t *testing.T) {
	t.Parallel()
	mr := NewMarkdownRenderer()

	content := "```go\nfmt.Println(\"hello\")\n```"
	rendered, err := mr.Render(content, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rendered == "" {
		t.Fatal("expected rendered content")
	}
	// Content should be rendered (contains ANSI codes, different from input)
	if rendered == content {
		t.Fatal("expected code to be rendered with formatting")
	}
}

func TestMarkdownRendererWithEmoji(t *testing.T) {
	t.Parallel()
	mr := NewMarkdownRenderer()

	content := "Hello :smile: world"
	rendered, err := mr.Render(content, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rendered == "" {
		t.Fatal("expected rendered content")
	}
}

func TestRenderPreviewWithMarkdown(t *testing.T) {
	t.Parallel()
	doc := PreviewDocument{
		SessionID: "test",
		Title:     "Test",
		Blocks: []PreviewBlock{
			{Kind: PreviewUser, Title: "User", Body: "hi"},
			{Kind: PreviewAssistant, Title: "Assistant", Body: "# Heading\n\n**Bold text**"},
		},
	}

	mr := NewMarkdownRenderer()
	rendered := RenderPreview(doc, 80, true, mr)

	if rendered == "" {
		t.Fatal("expected rendered preview")
	}
	if !strings.Contains(rendered, "Assistant") {
		t.Fatal("expected Assistant header")
	}
}

func TestRenderPreviewWithoutMarkdown(t *testing.T) {
	t.Parallel()
	doc := PreviewDocument{
		SessionID: "test",
		Title:     "Test",
		Blocks: []PreviewBlock{
			{Kind: PreviewUser, Title: "User", Body: "Plain text"},
		},
	}

	// Pass nil markdown renderer
	rendered := RenderPreview(doc, 80, false, nil)

	if rendered == "" {
		t.Fatal("expected rendered preview")
	}
	if !strings.Contains(rendered, "Plain text") {
		t.Fatal("expected plain text content")
	}
}
