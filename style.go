package main

import "github.com/charmbracelet/lipgloss"

const BORDER_WIDTH = 2

var (
	black     = lipgloss.Color("#000000")
	white     = lipgloss.Color("#FFFFFF")
	darkBlue  = lipgloss.Color("#1919A6")
	lightBlue = lipgloss.Color("#2121DE")
	yellow    = lipgloss.Color("#FFFF00")

	defaultStyle = lipgloss.NewStyle()
	windowStyle  = lipgloss.NewStyle()

	panelStyle = defaultStyle.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(darkBlue).
			Padding(0).
			Margin(0)

	tabStyle = defaultStyle.
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lightBlue).
			PaddingLeft(2).
			PaddingRight(2)

	selectedTabStyle = tabStyle.
				Foreground(yellow).
				UnsetBorderBottom().
				PaddingBottom(1)

	selectedStyle = defaultStyle.Background(white).Foreground(black).Padding(0).Margin(0)
	errorStyle    = defaultStyle.Foreground(lipgloss.Color("#FD0000"))
	successStyle  = defaultStyle.Foreground(lipgloss.Color("#00FF00"))

	reducedEmphasisStyle = defaultStyle.Foreground(lipgloss.Color("242"))
	hotkeyStyle          = reducedEmphasisStyle.Underline(true).PaddingLeft(1)
)
