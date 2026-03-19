package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/muesli/reflow/wordwrap"
)

type cachedPreview struct {
	modTime int64
	doc     PreviewDocument
}

// PreviewCache caches parsed previews by path and mtime.
type PreviewCache struct {
	mu    sync.Mutex
	items map[string]cachedPreview
}

// NewPreviewCache creates an in-memory preview cache.
func NewPreviewCache() *PreviewCache {
	return &PreviewCache{items: make(map[string]cachedPreview)}
}

// Load returns the cached preview if present or parses the rollout.
func (c *PreviewCache) Load(record SessionRecord) (PreviewDocument, error) {
	stat, err := os.Stat(record.Path)
	if err != nil {
		return PreviewDocument{}, fmt.Errorf("stat preview %s: %w", record.Path, err)
	}
	key := record.Path
	modTime := stat.ModTime().UnixNano()

	c.mu.Lock()
	if cached, ok := c.items[key]; ok && cached.modTime == modTime {
		c.mu.Unlock()
		return cached.doc, nil
	}
	c.mu.Unlock()

	doc, err := parsePreview(record)
	if err != nil {
		return PreviewDocument{}, fmt.Errorf("parse preview %s: %w", record.Path, err)
	}

	c.mu.Lock()
	c.items[key] = cachedPreview{modTime: modTime, doc: doc}
	c.mu.Unlock()
	return doc, nil
}

func parsePreview(record SessionRecord) (PreviewDocument, error) {
	file, err := os.Open(record.Path)
	if err != nil {
		return PreviewDocument{}, fmt.Errorf("open preview %s: %w", record.Path, err)
	}
	defer file.Close()

	doc := PreviewDocument{
		SessionID: record.ID,
		Title:     record.DisplayTitle(),
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var env recordEnvelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			continue
		}
		switch env.Type {
		case RecordTypeEventMsg:
			if block := eventBlock(env.Payload); block != nil {
				doc.Blocks = append(doc.Blocks, *block)
			}
		case RecordTypeResponseItem:
			if blocks := responseBlocks(env.Payload); len(blocks) > 0 {
				doc.Blocks = append(doc.Blocks, blocks...)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return PreviewDocument{}, fmt.Errorf("scan preview %s: %w", record.Path, err)
	}
	if len(doc.Blocks) == 0 {
		doc.Blocks = append(doc.Blocks, PreviewBlock{
			Kind:  PreviewEvent,
			Title: "No transcript",
			Body:  "This session has no renderable user or assistant messages yet.",
		})
	}
	return doc, nil
}

func eventBlock(raw json.RawMessage) *PreviewBlock {
	var payload eventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	switch payload.Type {
	case "user_message":
		if strings.TrimSpace(payload.Message) == "" {
			return nil
		}
		return &PreviewBlock{Kind: PreviewUser, Title: "User", Body: payload.Message}
	case "agent_message":
		if strings.TrimSpace(payload.Message) == "" {
			return nil
		}
		return &PreviewBlock{Kind: PreviewEvent, Title: "Agent", Body: payload.Message}
	case "task_started", "turn/completed", "turn_started":
		return &PreviewBlock{Kind: PreviewEvent, Title: payload.Type, Body: ""}
	default:
		return nil
	}
}

func responseBlocks(raw json.RawMessage) []PreviewBlock {
	var payload outputMessagePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	switch payload.Type {
	case "message":
		text := collectMessageText(payload.Content)
		if text == "" {
			return nil
		}
		title := "Assistant"
		if payload.Role == "developer" || payload.Role == "system" {
			title = "Context"
		}
		kind := PreviewAssistant
		if title == "Context" {
			kind = PreviewEvent
		}
		return []PreviewBlock{{Kind: kind, Title: title, Body: text}}
	case "function_call":
		body := shortenJSON(payload.Args, 200)
		return []PreviewBlock{{Kind: PreviewToolCall, Title: payload.Name, Body: body}}
	case "function_call_output":
		body := payload.Output
		if body == "" {
			body = "command completed"
		}
		return []PreviewBlock{{Kind: PreviewToolOut, Title: payload.CallID, Body: truncate(body, 500)}}
	default:
		return nil
	}
}

func collectMessageText(parts []messagePart) string {
	chunks := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Text == "" {
			continue
		}
		if part.Type == "input_text" || part.Type == "output_text" || part.Type == "text" {
			chunks = append(chunks, part.Text)
		}
	}
	return strings.TrimSpace(strings.Join(chunks, "\n"))
}

func shortenJSON(raw json.RawMessage, limit int) string {
	if len(raw) == 0 {
		return ""
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err == nil {
		return truncate(out.String(), limit)
	}
	return truncate(string(raw), limit)
}

func truncate(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	if limit <= 3 {
		return s[:limit]
	}
	return s[:limit-3] + "..."
}

// RenderPreview returns a wrapped preview suitable for a viewport.
// If mr is provided, assistant messages will be rendered as markdown with ANSI formatting.
func RenderPreview(doc PreviewDocument, width int, showSystem bool, mr *MarkdownRenderer) string {
	if width < 20 {
		width = 20
	}
	parts := make([]string, 0, len(doc.Blocks)+1)
	parts = append(parts, doc.Title)
	for _, block := range doc.Blocks {
		if !showSystem && block.Title == "Context" {
			continue
		}
		header := strings.TrimSpace(block.Title)
		if header == "" {
			header = string(block.Kind)
		}
		body := strings.TrimSpace(block.Body)
		if body == "" {
			parts = append(parts, fmt.Sprintf("[%s]", header))
			continue
		}

		var content string
		if mr != nil && block.Kind == PreviewAssistant {
			// Render assistant messages as markdown
			rendered, err := mr.Render(body, width-4) // account for borders/padding
			if err == nil {
				content = rendered
			} else {
				content = wordwrap.String(body, width)
			}
		} else {
			content = wordwrap.String(body, width)
		}
		parts = append(parts, fmt.Sprintf("[%s]\n%s", header, content))
	}
	return strings.Join(parts, "\n\n")
}
