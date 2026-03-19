# Interactive Scrollbar (TUI)

## Context

The TUI renders a one-column scrollbar for:

- Session list pane (`model.renderListPane()`)
- Grouped children pane (`model.renderChildPane()`)
- Preview viewport (`model.renderPreviewPane()`)

Today this scrollbar is visual-only. Mouse wheel scrolling is implemented, but clicking/dragging on the scrollbar has no effect (and clicks near the right edge can be misinterpreted as list-item clicks because `listIndexAtPosition` ignores `msg.X`).

## Goal

Make the scrollbar interactive with mouse input:

- Click on the scrollbar column to jump/seek the scroll position.
- Click-and-drag the scrollbar thumb to scroll continuously.
- Avoid changing the current selection when interacting with the **list** scrollbar (keep behavior aligned with mouse-wheel scrolling).

## Scope

- Wide mode: list + preview panes.
- Narrow mode: list pane and preview mode.
- Group detail mode: child list + preview panes.

## Proposed Behavior

### Hit testing

Treat the scrollbar as the last column of the pane content rect:

- `scrollbarX = contentRect.x + (componentWidth - 1)`
- `scrollbarY = contentRect.y + paneHeaderHeight`
- `scrollbarHeight = componentHeight`

Only enable interactions when the scrollbar is actually rendered (i.e., when content overflows and `RenderScrollbar` returns non-empty).

### Click-to-seek mapping

Map `msg.Y` onto a `percent` in `[0,1]` based on track space and thumb height:

- Convert click row to an intended `thumbPos` (center thumb around click).
- Convert `thumbPos` to `percent = thumbPos / trackSpace`.
- Apply to the correct scroll target:
  - List pane: `listScroll` in `[0, maxListScroll]`
  - Child list pane: selected index in `[0, len(items)-1]`
  - Preview pane: `viewport.YOffset` in `[0, totalLines-visibleLines]`

### Dragging

- Start drag when left-press occurs on a scrollbar.
- Update scroll on mouse motion while dragging (stop on left-release).
- Ignore item selection while dragging.

## Tests

Add TUI tests to cover:

- Clicking the scrollbar column updates scroll offset (list and preview).
- Clicking on scrollbar does not open preview/select item for list pane.
- Dragging updates offset across multiple motion events (at least one pane).

