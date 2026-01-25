package main

import (
	"strings"
	"time"

	cmd "ptui/command"
	"ptui/styles"
	"ptui/types"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type rootModel struct {
	selectedTab int

	tabs    []types.ChildModel
	spinner spinner.Model

	runningCommandsCount int

	termWidth  int
	termHeight int

	cmds []tea.Cmd
}

const (
	PackageList types.StreamTarget = iota
	PackageInfo
	Background
)

var (
	yellow       = styles.Yellow
	defaultStyle = styles.DefaultStyle
	darkBlue     = styles.DarkBlue

	panelStyle           = styles.PanelStyle
	reducedEmphasisStyle = styles.ReducedEmphasisStyle
	selectedStyle        = styles.SelectedStyle
	windowStyle          = styles.WindowStyle
	tabStyle             = styles.TabStyle
	selectedTabStyle     = styles.SelectedTabStyle

	standardBorder    = styles.RoundedBorder
	topLeftBorder     = styles.TopLeftBorder
	topRightBorder    = styles.TopRightBorder
	bottomLeftBorder  = styles.BottomLeftBorder
	bottomRightBorder = styles.BottomRightBorder
	horizontalBorder  = styles.HorizontalBorder
	verticalBorder    = styles.VerticalBorder

	BORDER_WIDTH = styles.BORDER_WIDTH
)

var header = defaultStyle.Foreground(yellow).Render(`
 ██████╗  ████████╗ ██╗   ██╗ ██╗
 ██╔══██╗ ╚══██╔══╝ ██║   ██║ ██║
 ██████╔╝    ██║    ██║   ██║ ██║
 ██╔═══╝     ██║    ██║   ██║ ██║
 ██║         ██║    ╚██████╔╝ ██║
 ╚═╝         ╚═╝     ╚═════╝  ╚═╝`)

func initialModel() *rootModel {
	installedTab := initialInstalledModel()
	browseTab := initialBrowseModel()

	spinner := spinner.New(
		spinner.WithSpinner(
			spinner.Spinner{
				Frames: []string{
					defaultStyle.Foreground(yellow).Render(" ●"),
					defaultStyle.Foreground(yellow).Render("  𜱭"),
					defaultStyle.Foreground(yellow).Render("   ●"),
					defaultStyle.Foreground(yellow).Render("    𜱭"),
				},
				FPS: time.Second / 3,
			}))

	return &rootModel{
		selectedTab: 0,
		tabs:        []types.ChildModel{installedTab, browseTab},
		spinner:     spinner,
		cmds:        make([]tea.Cmd, 0, 6),
	}
}

func (m *rootModel) Init() tea.Cmd {
	return tea.Batch(m.InitSelectedTab(), tea.SetWindowTitle(APP_NAME))
}

func (m *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.cmds = m.cmds[:0]

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.runningCommandsCount > 0 {
			updated, cmd := m.spinner.Update(msg)
			m.spinner = updated
			m.cmds = append(m.cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.termWidth, m.termHeight = msg.Width, msg.Height

		for i, tab := range m.tabs {
			updated, cmd := tab.Update(
				types.ContentRectMsg{
					Width:  msg.Width,
					Height: msg.Height - 16},
			)

			m.tabs[i] = updated.(types.ChildModel)
			if cmd != nil {
				m.cmds = append(m.cmds, cmd)
			}
		}

	case types.HotkeyPressedMsg:
		m.cmds = append(m.cmds, msg.Hotkey.Command())

	case cmd.CommandStartMsg, cmd.CommandChunkMsg, cmd.CommandDoneMsg, installedInitMsg, browseInitMsg:
		switch msg := msg.(type) {
		case cmd.CommandStartMsg:
			if isLongRunning(msg.Target) {
				m.runningCommandsCount++
				m.cmds = append(m.cmds, m.spinner.Tick)
			}

		case cmd.CommandDoneMsg:
			if isLongRunning(msg.Target) {
				m.runningCommandsCount--
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab":
			if m.selectedTab < len(m.tabs)-1 {
				m.selectedTab++
				m.cmds = append(m.cmds, m.InitSelectedTab())
			}

		case "shift+tab":
			if m.selectedTab > 0 {
				m.selectedTab--
				m.cmds = append(m.cmds, m.InitSelectedTab())
			}
		}
	}

	updated, cmd := m.tabs[m.selectedTab].Update(msg)
	m.tabs[m.selectedTab] = updated.(types.ChildModel)
	if cmd != nil {
		m.cmds = append(m.cmds, cmd)
	}

	return m, tea.Batch(m.cmds...)
}

func (m *rootModel) View() string {
	titlePanel := panelStyle.Width(m.termWidth - BORDER_WIDTH).Render(header)

	var renderedTabs []string
	for i, tab := range m.tabs {
		renderedTabs = append(renderedTabs, renderTab(m, tab.Title(), i))
	}

	tabPanel := windowStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, renderedTabs...))

	view := lipgloss.JoinVertical(lipgloss.Left, titlePanel, tabPanel)

	tabView := m.tabs[m.selectedTab].View()

	lengthToSelectedTabStart := lipgloss.Width(strings.Join(renderedTabs[0:m.selectedTab], ""))
	tabView = withTabConnectorTopBorder(tabView, lengthToSelectedTabStart, lipgloss.Width(renderedTabs[m.selectedTab]))

	view = lipgloss.JoinVertical(lipgloss.Left, view, tabView)

	return windowStyle.MaxWidth(m.termWidth).Height(m.termHeight).Render(view)
}

func (m *rootModel) InitSelectedTab() tea.Cmd {
	return m.tabs[m.selectedTab].Init()
}

func renderTab(m *rootModel, title string, index int) (renderedTab string) {
	if m.selectedTab == index {
		renderedTab = selectedTabStyle.Render(title)
	} else {
		renderedTab = tabStyle.Render(reducedEmphasisStyle.Render(title))
	}
	return renderedTab
}
