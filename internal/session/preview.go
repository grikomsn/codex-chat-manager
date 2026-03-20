package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
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
	toolNames := make(map[string]string)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var env recordEnvelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			continue
		}
		switch env.Type {
		case RecordTypeEventMsg, RecordTypeEventMsgHyphen:
			if block := eventBlock(env.Payload); block != nil {
				doc.Blocks = appendBlockDedup(doc.Blocks, *block)
			}
		case RecordTypeResponseItem, RecordTypeResponseItemHyphen:
			if blocks := responseBlocks(env.Payload, toolNames); len(blocks) > 0 {
				for _, block := range blocks {
					doc.Blocks = appendBlockDedup(doc.Blocks, block)
				}
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

func appendBlockDedup(blocks []PreviewBlock, next PreviewBlock) []PreviewBlock {
	if len(blocks) == 0 {
		return append(blocks, next)
	}
	last := blocks[len(blocks)-1]
	if last.Kind == next.Kind && last.Title == next.Title && strings.TrimSpace(last.Body) == strings.TrimSpace(next.Body) {
		return blocks
	}
	return append(blocks, next)
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
		if isInjectedAgentsContext(payload.Message) {
			return &PreviewBlock{Kind: PreviewEvent, Title: "Context", Body: truncate(payload.Message, injectedAgentsContextPreviewLimit)}
		}
		return &PreviewBlock{Kind: PreviewUser, Title: "User", Body: payload.Message}
	case "agent_message":
		if strings.TrimSpace(payload.Message) == "" {
			return nil
		}
		if isInjectedAgentsContext(payload.Message) {
			return &PreviewBlock{Kind: PreviewEvent, Title: "Context", Body: truncate(payload.Message, injectedAgentsContextPreviewLimit)}
		}
		return &PreviewBlock{Kind: PreviewAssistant, Title: "Assistant", Body: payload.Message}
	case "task_started":
		return nil
	case "turn/completed", "turn_started":
		return &PreviewBlock{Kind: PreviewEvent, Title: payload.Type, Body: ""}
	default:
		return nil
	}
}

func cmdFromToolArgs(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	type cmdCarrier struct {
		Cmd string `json:"cmd"`
	}

	var obj cmdCarrier
	if err := json.Unmarshal(raw, &obj); err == nil && strings.TrimSpace(obj.Cmd) != "" {
		cmd := strings.TrimSpace(obj.Cmd)
		if looksLikeJSONText(cmd) {
			var nested cmdCarrier
			if json.Unmarshal([]byte(cmd), &nested) == nil && strings.TrimSpace(nested.Cmd) != "" {
				return strings.TrimSpace(nested.Cmd)
			}
		}
		return cmd
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return ""
		}
		if json.Unmarshal([]byte(asString), &obj) == nil && strings.TrimSpace(obj.Cmd) != "" {
			cmd := strings.TrimSpace(obj.Cmd)
			if looksLikeJSONText(cmd) {
				var nested cmdCarrier
				if json.Unmarshal([]byte(cmd), &nested) == nil && strings.TrimSpace(nested.Cmd) != "" {
					return strings.TrimSpace(nested.Cmd)
				}
			}
			return cmd
		}
	}
	return ""
}

func stringFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}
	return strings.TrimSpace(string(raw))
}

func normalizeToolOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return output
	}
	if !looksLikeJSONText(trimmed) {
		return output
	}
	var carrier struct {
		Output string `json:"output"`
	}
	if json.Unmarshal([]byte(trimmed), &carrier) == nil && strings.TrimSpace(carrier.Output) != "" {
		return strings.TrimSpace(carrier.Output)
	}
	return output
}

func responseBlocks(raw json.RawMessage, toolNames map[string]string) []PreviewBlock {
	var payload outputMessagePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	switch payload.Type {
	case "message":
		text := collectMessageText(payload.Content)
		if text == "" {
			text = strings.TrimSpace(payload.Message)
		}
		if text == "" {
			return nil
		}
		switch payload.Role {
		case "developer", "system":
			return []PreviewBlock{{Kind: PreviewEvent, Title: "Context", Body: text}}
		case "user":
			if isInjectedAgentsContext(text) {
				return []PreviewBlock{{Kind: PreviewEvent, Title: "Context", Body: truncate(text, injectedAgentsContextPreviewLimit)}}
			}
			return []PreviewBlock{{Kind: PreviewUser, Title: "User", Body: text}}
		default:
			if isInjectedAgentsContext(text) {
				return []PreviewBlock{{Kind: PreviewEvent, Title: "Context", Body: truncate(text, injectedAgentsContextPreviewLimit)}}
			}
			return []PreviewBlock{{Kind: PreviewAssistant, Title: "Assistant", Body: text}}
		}
	case "function_call", "tool_call", "custom_tool_call":
		body := stringFromRaw(payload.Input)
		if body == "" {
			body = cmdFromToolArgs(payload.Args)
		}
		if body == "" {
			if strings.TrimSpace(payload.Name) == "exec_command" {
				body = shortenJSON(payload.Args, 200)
			} else if len(payload.Input) > 0 {
				body = shortenJSON(payload.Input, 400)
			} else {
				body = shortenJSON(payload.Args, 200)
			}
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			name = "Tool Call"
		}
		title := name
		if payload.CallID != "" {
			toolNames[payload.CallID] = name
			title = fmt.Sprintf("%s (%s)", name, payload.CallID)
		}
		return []PreviewBlock{{Kind: PreviewToolCall, Title: title, Body: body}}
	case "function_call_output", "tool_call_output", "tool_output", "custom_tool_call_output":
		body := normalizeToolOutput(payload.Output)
		if body == "" {
			body = "command completed"
		}
		callID := strings.TrimSpace(payload.CallID)
		title := callID
		if callID == "" {
			title = "Tool Output"
		} else if name := strings.TrimSpace(toolNames[callID]); name != "" {
			title = fmt.Sprintf("%s (%s)", name, callID)
		}
		return []PreviewBlock{{Kind: PreviewToolOut, Title: title, Body: truncate(body, 500)}}
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

func codeFence(lang, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		if lang == "" {
			return "```\n```"
		}
		return "```" + lang + "\n```"
	}
	maxRun := 0
	run := 0
	for _, r := range content {
		if r == '`' {
			run++
			continue
		}
		if run > maxRun {
			maxRun = run
		}
		run = 0
	}
	if run > maxRun {
		maxRun = run
	}
	fenceLen := 3
	if maxRun >= fenceLen {
		fenceLen = maxRun + 1
	}
	fence := strings.Repeat("`", fenceLen)
	if lang != "" {
		return fmt.Sprintf("%s%s\n%s\n%s", fence, lang, content, fence)
	}
	return fmt.Sprintf("%s\n%s\n%s", fence, content, fence)
}

func padRightANSI(s string, width int) string {
	s = ansi.Truncate(s, width, "")
	pad := width - ansi.StringWidth(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func looksLikeJSONText(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

func renderBubble(label, content string, outerWidth, leftPad int) string {
	if outerWidth < 20 {
		outerWidth = 20
	}
	if leftPad < 0 {
		leftPad = 0
	}
	innerWidth := outerWidth - 2
	if innerWidth < 8 {
		innerWidth = 8
		outerWidth = innerWidth + 2
	}

	label = strings.TrimSpace(label)
	if label == "" {
		label = "Message"
	}

	labelMax := outerWidth - 5
	if labelMax < 0 {
		labelMax = 0
	}
	if ansi.StringWidth(label) > labelMax {
		label = ansi.Truncate(label, labelMax, "…")
	}
	dashLen := outerWidth - (ansi.StringWidth(label) + 5)
	if dashLen < 0 {
		dashLen = 0
	}

	prefix := strings.Repeat(" ", leftPad)
	lines := make([]string, 0, 4)
	lines = append(lines, prefix+"╭─ "+label+" "+strings.Repeat("─", dashLen)+"╮")

	content = strings.TrimRight(content, "\n")
	bodyLines := []string{""}
	if strings.TrimSpace(content) != "" {
		bodyLines = strings.Split(content, "\n")
	}
	for _, line := range bodyLines {
		line = padRightANSI(line, innerWidth)
		lines = append(lines, prefix+"│"+line+"│")
	}
	lines = append(lines, prefix+"╰"+strings.Repeat("─", outerWidth-2)+"╯")
	return strings.Join(lines, "\n")
}

func computeBubbleLayout(width int) (availableWidth, bubbleWidth int) {
	availableWidth = width
	if availableWidth > 2 {
		availableWidth -= 2
	}
	if availableWidth < 20 {
		availableWidth = 20
	}

	bubbleWidth = int(float64(availableWidth) * 0.92)
	if bubbleWidth < 20 {
		bubbleWidth = 20
	}
	if bubbleWidth > availableWidth {
		bubbleWidth = availableWidth
	}
	return availableWidth, bubbleWidth
}

func filterPreviewBlocks(blocks []PreviewBlock, showSystem bool) []PreviewBlock {
	if showSystem {
		return blocks
	}
	out := make([]PreviewBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Title == "Context" {
			continue
		}
		out = append(out, block)
	}
	return out
}

func bubbleLabelAndLang(kind PreviewBlockKind, header, body string) (string, string) {
	switch kind {
	case PreviewToolCall:
		lowerHeader := strings.ToLower(header)
		if strings.Contains(body, "*** Begin Patch") || strings.HasPrefix(lowerHeader, "apply_patch") {
			return "Tool: " + header, "diff"
		}
		if looksLikeJSONText(body) {
			return "Tool: " + header, "json"
		}
		return "Tool: " + header, "bash"
	case PreviewToolOut:
		if looksLikeJSONText(body) {
			return "Output: " + header, "json"
		}
		return "Output: " + header, "text"
	case PreviewEvent:
		if header == "Context" {
			return header, "text"
		}
		return header, ""
	default:
		return header, ""
	}
}

func renderBlockBody(body, lang string, mr *MarkdownRenderer, width int) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	if mr == nil {
		return wordwrap.String(body, width)
	}
	markdown := body
	if lang != "" {
		markdown = codeFence(lang, body)
	}
	rendered, err := mr.Render(markdown, width)
	if err != nil {
		return wordwrap.String(body, width)
	}
	return rendered
}

func flattenOneLine(s string) (string, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", false
	}
	hasNewline := strings.Contains(trimmed, "\n")
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", hasNewline
	}
	return strings.Join(parts, " "), hasNewline
}

func stripCallIDSuffix(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	if idx := strings.LastIndex(title, " (call"); idx > 0 && strings.HasSuffix(title, ")") {
		return strings.TrimSpace(title[:idx])
	}
	return title
}

func execCommandOutputBody(output string) string {
	const marker = "\nOutput:\n"
	if idx := strings.Index(output, marker); idx >= 0 {
		if body := strings.TrimSpace(output[idx+len(marker):]); body != "" {
			return body
		}
	}
	return output
}

func renderToolLine(kind PreviewBlockKind, title, body string, width int) string {
	if width < 20 {
		width = 20
	}
	if kind == PreviewToolCall && looksLikeJSONText(body) && strings.Contains(body, "\"cmd\"") {
		if cmd := cmdFromToolArgs(json.RawMessage(body)); cmd != "" {
			body = cmd
		}
	}
	title = stripCallIDSuffix(title)
	if strings.EqualFold(strings.TrimSpace(title), "exec_command") {
		title = ""
	}
	if kind == PreviewToolOut && title == "" {
		body = execCommandOutputBody(body)
	}
	flat, multiline := flattenOneLine(body)
	if flat == "" {
		flat = "…"
	}

	prefix := "> "
	if kind == PreviewToolOut {
		prefix = "< "
	}

	sep := ": "
	if strings.TrimSpace(title) == "" {
		sep = ""
	}
	minCmd := 10
	maxTitleWidth := width - ansi.StringWidth(prefix) - ansi.StringWidth(sep) - minCmd
	if maxTitleWidth <= 0 {
		title = ""
		sep = ""
	} else if ansi.StringWidth(title) > maxTitleWidth {
		title = ansi.Truncate(title, maxTitleWidth, "…")
	}

	remainingCmd := width - ansi.StringWidth(prefix) - ansi.StringWidth(title) - ansi.StringWidth(sep)
	if remainingCmd < 0 {
		remainingCmd = 0
	}
	cmdPart := ansi.Truncate(flat, remainingCmd, "…")
	line := prefix + title + sep + cmdPart
	if multiline && remainingCmd >= 2 {
		line = ansi.Truncate(line+" …", width, "…")
	}
	return ansi.Truncate(line, width, "…")
}

// RenderPreview returns a wrapped preview suitable for a viewport.
// If mr is provided, individual messages will be rendered as markdown with ANSI formatting.
func RenderPreview(doc PreviewDocument, width int, showSystem bool, mr *MarkdownRenderer) string {
	if width < 20 {
		width = 20
	}

	parts := make([]string, 0, len(doc.Blocks)+2)
	parts = append(parts, strings.TrimSpace(doc.Title))

	blocks := filterPreviewBlocks(doc.Blocks, showSystem)

	availableWidth, bubbleWidth := computeBubbleLayout(width)

	for _, block := range blocks {
		header := strings.TrimSpace(block.Title)
		if header == "" {
			header = string(block.Kind)
		}
		body := strings.TrimSpace(block.Body)

		if block.Kind == PreviewToolCall || block.Kind == PreviewToolOut {
			parts = append(parts, renderToolLine(block.Kind, header, body, width))
			continue
		}

		label, lang := bubbleLabelAndLang(block.Kind, header, body)

		outerWidth := bubbleWidth
		leftPad := 0
		if block.Kind == PreviewUser {
			leftPad = availableWidth - bubbleWidth
			if leftPad < 0 {
				leftPad = 0
			}
		}
		innerWidth := outerWidth - 2

		content := renderBlockBody(body, lang, mr, innerWidth)

		parts = append(parts, renderBubble(label, content, outerWidth, leftPad))
	}
	return strings.Join(parts, "\n\n")
}
