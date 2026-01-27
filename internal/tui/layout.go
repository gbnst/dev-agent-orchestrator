package tui

// Region defines a rectangular area within the terminal.
type Region struct {
	X      int // Left position (0-indexed)
	Y      int // Top position (0-indexed)
	Width  int // Width in cells
	Height int // Height in lines
}

// Layout holds computed regions for all UI components.
type Layout struct {
	Header    Region // App title and subtitle (2 lines)
	Tabs      Region // Tab bar (1 line)
	Content   Region // Main content area (dynamic)
	Logs      Region // Log panel when open (dynamic, 60% of content area)
	StatusBar Region // Status bar (1 line)
	Separator Region // Separator between content and logs (1 line when logs open)
}

// Fixed heights for chrome elements
const (
	headerHeight    = 2 // Title + subtitle
	tabsHeight      = 1 // Tab bar
	statusBarHeight = 1 // Status bar
	marginHeight    = 2 // Top + bottom margins
	separatorHeight = 1 // Separator when log panel open
)

// ComputeLayout calculates regions based on terminal dimensions.
// When logPanelOpen is true, content area splits 40/60 (content/logs).
func ComputeLayout(width, height int, logPanelOpen bool) Layout {
	// Calculate available height for dynamic content
	fixedHeight := headerHeight + tabsHeight + statusBarHeight + marginHeight
	availableHeight := height - fixedHeight

	// Ensure minimum usable height
	if availableHeight < 4 {
		availableHeight = 4
	}

	var contentHeight, logsHeight int
	if logPanelOpen {
		// When logs are open, split available height 40/60
		// The separator is added on top of the layout
		contentHeight = int(float64(availableHeight) * 0.4)
		logsHeight = availableHeight - contentHeight
	} else {
		contentHeight = availableHeight
		logsHeight = 0
	}

	// Build layout top-to-bottom
	y := 0

	header := Region{X: 0, Y: y, Width: width, Height: headerHeight}
	y += headerHeight

	tabs := Region{X: 0, Y: y, Width: width, Height: tabsHeight}
	y += tabsHeight

	content := Region{X: 0, Y: y, Width: width, Height: contentHeight}
	y += contentHeight

	var separator, logs Region
	if logPanelOpen {
		separator = Region{X: 0, Y: y, Width: width, Height: separatorHeight}
		y += separatorHeight

		logs = Region{X: 0, Y: y, Width: width, Height: logsHeight}
		y += logsHeight
	}

	statusBar := Region{X: 0, Y: y, Width: width, Height: statusBarHeight}

	return Layout{
		Header:    header,
		Tabs:      tabs,
		Content:   content,
		Logs:      logs,
		StatusBar: statusBar,
		Separator: separator,
	}
}

// ContentListHeight returns the height available for the container/session list
// after accounting for list chrome (selection indicator, padding).
func (l Layout) ContentListHeight() int {
	// Subtract 2 for list padding/borders
	h := l.Content.Height - 2
	if h < 1 {
		h = 1
	}
	return h
}
