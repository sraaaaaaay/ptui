package main

import (
	"math"
	"strings"
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
