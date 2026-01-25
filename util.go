package main

import (
	"math"
	"ptui/types"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func createScrollbar(width int, pos int, listLength int, contentHeight int, listLoaded bool) string {
	n := max(1, listLength)
	size := max(1, int(math.Round(float64(contentHeight)/float64(n))))

	blocks := strings.Repeat("█", width)
	var scrollBar strings.Builder
	for i := range size {
		if i < size-1 {
			scrollBar.WriteString(blocks + "\n")
		} else {
			scrollBar.WriteString(blocks)
		}
	}

	str := scrollBar.String()
	if listLoaded {
		yRelative := math.Round(
			float64(pos) *
				float64(contentHeight) /
				float64(listLength))

		str = defaultStyle.PaddingTop(int(yRelative)).Render(str)
	}

	return str
}

func matchesSearch(text string, search string) (match bool) {
	if search == "" || strings.Contains(text, search) {
		return true
	}

	return false
}

func scrollIntoView(vp *viewport.Model, i int) {
	if i < vp.YOffset {
		vp.SetYOffset(i)
	}

	end := vp.YOffset + vp.VisibleLineCount()
	if i >= end-1 {
		vp.ScrollDown(i - end + 1)
	}
}

func buildSortedHotkeyList(vp *viewport.Model, hotkeys map[string]types.HotkeyBinding, keysOrdered []string) {
	var list strings.Builder
	for _, key := range keysOrdered {
		hotkey := hotkeys[key]

		skWidth := lipgloss.Width(hotkey.Shortcut)
		dWidth := lipgloss.Width(hotkey.Description)

		paddingWidth := max(0, vp.Width-skWidth-dWidth-1)

		list.WriteString(reducedEmphasisStyle.Render(hotkey.Description))
		list.WriteString(strings.Repeat(" ", paddingWidth))
		list.WriteString(hotkey.Shortcut)
		list.WriteRune('\n')
	}

	vp.SetContent(list.String())
}

func handleHotkeyAndSearch(m types.PackageListModel, msg tea.KeyMsg) {
	msgConsumed := false
	msgStr := msg.String()

	hotkeys := m.Hotkeys()
	searchInput := m.SearchInput()

	// In specific cases, "global" hotkeys should be consumed by the program
	// rather than passed to the text input. If hotkeys are ever made configurable
	// there'll need to be a way to resolve which one toggles the search.
	if msgStr == "/" {
		if hotkey, exists := hotkeys[msgStr]; exists {
			m.AddCommand(func() tea.Msg { return types.HotkeyPressedMsg{Hotkey: hotkey} })
			msgConsumed = true
		}
	}

	if !msgConsumed && searchInput.Focused() {
		oldVal := searchInput.Value()
		updated, cmd := searchInput.Update(msg)
		newVal := updated.Value()

		*searchInput = updated
		if cmd != nil {
			m.AddCommand(cmd)
		}

		if oldVal != newVal {
			m.ResetCursor()
		}
		msgConsumed = true
	}

	if !msgConsumed {
		hotkey, exists := hotkeys[msgStr]
		if exists {
			m.AddCommand(func() tea.Msg { return types.HotkeyPressedMsg{Hotkey: hotkey} })
		}
	}
}

func createCustomBottomBorder(content string, bottomBorderContent string, drawTop bool) string {
	contentWithDefaultSideBorders := defaultStyle.
		BorderForeground(darkBlue).
		Border(standardBorder, drawTop, true, false, true).
		Render(content)

	// Subtract a space here because having one extra border character
	// between the custom content and the bottom-right corner looks better.
	contentWidth := lipgloss.Width(content)
	bottomBorderContentPadding := contentWidth - lipgloss.Width(bottomBorderContent) - 1

	customBottomBorder := bottomLeftBorder +
		strings.Repeat(horizontalBorder, bottomBorderContentPadding) +
		bottomBorderContent +
		horizontalBorder +
		bottomRightBorder

	return lipgloss.JoinVertical(lipgloss.Left, contentWithDefaultSideBorders, customBottomBorder)
}

func withTabConnectorTopBorder(content string, emptySectionStart int, emptySectionLength int) string {
	contentWidth := lipgloss.Width(content)
	emptySectionEnd := emptySectionStart + emptySectionLength

	var borderStart string
	var tabConnectorStart string
	if emptySectionStart == 0 {
		borderStart = verticalBorder
		tabConnectorStart = verticalBorder
	} else {
		borderStart = topLeftBorder
		tabConnectorStart = bottomRightBorder
	}

	// TODO make this less stupid
	var customTopBorder strings.Builder
	for position := range contentWidth {
		if position == 0 {
			customTopBorder.WriteString(borderStart)
		} else if position < emptySectionStart {
			customTopBorder.WriteString(horizontalBorder)
		} else if position == emptySectionStart {
			customTopBorder.WriteString(tabConnectorStart)
		} else if position == emptySectionEnd-1 {
			customTopBorder.WriteString(bottomLeftBorder)
		} else if position > emptySectionStart && position < emptySectionStart+emptySectionLength {
			customTopBorder.WriteString(" ")
		} else if position == contentWidth-1 {
			customTopBorder.WriteString(topRightBorder)
		} else {
			customTopBorder.WriteString(horizontalBorder)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, customTopBorder.String(), content)
}

func isUrl(str string) bool {
	return strings.Contains(str, "https") || strings.Contains(str, "http")
}

func isLongRunning(t types.StreamTarget) bool {
	switch t {
	case Background:
		return true
	default:
		return false
	}
}
