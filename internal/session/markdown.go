package session

import (
	"sync"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/lipgloss"
)

// MarkdownRenderer provides cached markdown rendering with terminal-aware styling.
type MarkdownRenderer struct {
	mu        sync.RWMutex
	renderers map[int]*glamour.TermRenderer
	styleOpt  glamour.TermRendererOption
}

// NewMarkdownRenderer creates a new markdown renderer with auto-detected terminal style.
func NewMarkdownRenderer() *MarkdownRenderer {
	var styleOpt glamour.TermRendererOption
	if lipgloss.HasDarkBackground() {
		styleOpt = glamour.WithStandardStyle("dark")
	} else {
		styleOpt = glamour.WithStandardStyle("light")
	}

	return &MarkdownRenderer{
		renderers: make(map[int]*glamour.TermRenderer),
		styleOpt:  styleOpt,
	}
}

// Render converts markdown content to ANSI-formatted terminal output.
// Width specifies the maximum line width for wrapping.
func (mr *MarkdownRenderer) Render(content string, width int) (string, error) {
	if width < 20 {
		width = 20
	}

	mr.mu.RLock()
	r, exists := mr.renderers[width]
	mr.mu.RUnlock()

	if !exists {
		var err error
		r, err = glamour.NewTermRenderer(
			mr.styleOpt,
			glamour.WithWordWrap(width),
			glamour.WithEmoji(),
		)
		if err != nil {
			return "", err
		}
		mr.mu.Lock()
		mr.renderers[width] = r
		mr.mu.Unlock()
	}

	return r.Render(content)
}

// ClearCache removes all cached renderers. Call this on terminal resize.
func (mr *MarkdownRenderer) ClearCache() {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.renderers = make(map[int]*glamour.TermRenderer)
}
