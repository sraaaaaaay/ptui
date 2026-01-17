package main

import (
	"math"
	"ptui/types"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
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
