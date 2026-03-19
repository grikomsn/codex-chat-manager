package tui

import (
	"github.com/charmbracelet/lipgloss"
)

type ScrollbarStyle struct {
	Track lipgloss.Style
	Thumb lipgloss.Style
}

func (s ScrollbarStyle) RenderScrollbar(percent float64, contentLines, visibleLines, height int) string {
	if contentLines <= visibleLines || height <= 0 {
		return ""
	}

	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}

	thumbHeight := max(1, height*visibleLines/contentLines)
	if thumbHeight > height {
		thumbHeight = height
	}

	trackSpace := height - thumbHeight
	if trackSpace <= 0 {
		trackSpace = 0
	}

	thumbPos := int(float64(trackSpace) * percent)
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos > trackSpace {
		thumbPos = trackSpace
	}

	trackChar := "░"
	thumbChar := "█"

	var lines []string
	for i := 0; i < height; i++ {
		if i >= thumbPos && i < thumbPos+thumbHeight {
			lines = append(lines, s.Thumb.Render(thumbChar))
		} else {
			lines = append(lines, s.Track.Render(trackChar))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
