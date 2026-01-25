package styles

import "github.com/charmbracelet/lipgloss"

const BORDER_WIDTH = 2

var (
	Black     = lipgloss.Color("#000000")
	White     = lipgloss.Color("#FFFFFF")
	DarkBlue  = lipgloss.Color("#1919A6")
	LightBlue = lipgloss.Color("#2121DE")
	Yellow    = lipgloss.Color("#FFFF00")
	DarkGrey  = lipgloss.Color("#333333")

	DefaultStyle = lipgloss.NewStyle()
	WindowStyle  = lipgloss.NewStyle()

	RoundedBorder = lipgloss.RoundedBorder()

	// Lipgloss doesn't natively support multicoloured borders, making it useless for
	// displaying informational text inline with the border. Instead, pre-render
	// some of the characters and use them to build custom borders as part of the view
	TopLeftBorder     = DefaultStyle.Foreground(DarkBlue).Render(RoundedBorder.TopLeft)
	TopRightBorder    = DefaultStyle.Foreground(DarkBlue).Render(RoundedBorder.TopRight)
	BottomLeftBorder  = DefaultStyle.Foreground(DarkBlue).Render(RoundedBorder.BottomLeft)
	BottomRightBorder = DefaultStyle.Foreground(DarkBlue).Render(RoundedBorder.BottomRight)

	HorizontalBorder = DefaultStyle.Foreground(DarkBlue).Render(RoundedBorder.Top)
	VerticalBorder   = DefaultStyle.Foreground(DarkBlue).Render(RoundedBorder.Left)

	PanelStyle = DefaultStyle.
			Border(RoundedBorder).
			BorderForeground(DarkBlue).
			Padding(0).
			Margin(0)

	TabStyle = DefaultStyle.
			Border(RoundedBorder).
			BorderForeground(LightBlue).
			PaddingLeft(2).
			PaddingRight(2)

	SelectedTabStyle = TabStyle.
				Foreground(Yellow).
				UnsetBorderBottom().
				PaddingBottom(1)

	SelectedStyle = DefaultStyle.Background(White).Foreground(Black).Padding(0).Margin(0)
	ErrorStyle    = DefaultStyle.Foreground(lipgloss.Color("#FD0000"))
	SuccessStyle  = DefaultStyle.Foreground(lipgloss.Color("#00FF00"))

	ReducedEmphasisStyle = DefaultStyle.Foreground(lipgloss.Color("242"))
	HotkeyStyle          = ReducedEmphasisStyle.Underline(true).PaddingLeft(1)
)
