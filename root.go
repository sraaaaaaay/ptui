package main

import (
	"os"
	"time"

	cmd "ptui/command"
	"ptui/types"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type rootModel struct {
	selectedTab int

	tabs    []types.ChildModel
	spinner spinner.Model

	isExecutingCommand   bool
	executingCommandName string

	termWidth  int
	termHeight int
	panelWidth int

	cmds []tea.Cmd
}

const (
	PackageList types.StreamTarget = iota
	PackageInfo
	Background
	SearchResultList
)

var header = defaultStyle.Foreground(yellow).Render(`
 ██████╗  ████████╗ ██╗   ██╗ ██╗
 ██╔══██╗ ╚══██╔══╝ ██║   ██║ ██║
 ██████╔╝    ██║    ██║   ██║ ██║
 ██╔═══╝     ██║    ██║   ██║ ██║
 ██║         ██║    ╚██████╔╝ ██║
 ╚═╝         ╚═╝     ╚═════╝  ╚═╝`)

var dump, _ = os.OpenFile("messages.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)

func initialModel() rootModel {
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

	return rootModel{
		selectedTab: 0,
		tabs:        []types.ChildModel{installedTab, browseTab},
		spinner:     spinner,
		cmds:        make([]tea.Cmd, 0, 6),
	}
}

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(m.InitSelectedTab(), tea.SetWindowTitle(APP_NAME))
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	m.cmds = m.cmds[:0]

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.isExecutingCommand {
			updated, cmd := m.spinner.Update(msg)
			m.spinner = updated
			m.cmds = append(m.cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.termWidth, m.termHeight = msg.Width, msg.Height
		m.panelWidth = int(float64(msg.Width) * 0.5)

		for i, tab := range m.tabs {
			updated, cmd := tab.Update(
				types.ContentRectMsg{
					Width:  msg.Width - 3*BORDER_WIDTH,
					Height: msg.Height - 17},
			)

			m.tabs[i] = updated.(types.ChildModel)
			if cmd != nil {
				m.cmds = append(m.cmds, cmd)
			}
		}

	case types.HotkeyPressedMsg:
		m.executingCommandName = msg.Hotkey.Description
		m.cmds = append(m.cmds, msg.Hotkey.Command())

	case cmd.CommandStartMsg, cmd.CommandChunkMsg, cmd.CommandDoneMsg, installedInitMsg, browseInitMsg:
		switch msg := msg.(type) {
		case cmd.CommandStartMsg:
			if msg.IsLongRunning {
				m.isExecutingCommand = true
				m.cmds = append(m.cmds, m.spinner.Tick)
			}

		case cmd.CommandDoneMsg:
			m.isExecutingCommand = false
			m.executingCommandName = ""
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

func (m rootModel) View() string {
	totalWidth := m.termWidth - BORDER_WIDTH
	titlePanel := panelStyle.Render(lipgloss.Place(
		totalWidth,
		lipgloss.Height(header)-BORDER_WIDTH-1,
		lipgloss.Center,
		lipgloss.Center,
		header,
	))

	installedTab := renderTab(m, "Installed", 0)
	browseTab := renderTab(m, "Browse", 1)

	tabPanel := windowStyle.Render(lipgloss.JoinHorizontal(lipgloss.Left, installedTab, browseTab))

	view := lipgloss.JoinVertical(lipgloss.Left, titlePanel, tabPanel)
	tabView := lipgloss.PlaceHorizontal(totalWidth, lipgloss.Left, m.tabs[m.selectedTab].View())

	statusBar := m.tabs[m.selectedTab].StatusBar()

	tabView = panelStyle.Render(lipgloss.JoinVertical(lipgloss.Left, tabView, statusBar))
	view = lipgloss.JoinVertical(lipgloss.Center, view, tabView)

	return windowStyle.Width(m.termWidth).Height(m.termHeight).Render(view)
}

func (m *rootModel) InitSelectedTab() tea.Cmd {
	return m.tabs[m.selectedTab].Init()
}

func renderTab(m rootModel, title string, index int) (result string) {
	if m.selectedTab == index {
		result = selectedTabStyle.Render(title)
	} else {
		result = tabStyle.Render(reducedEmphasisStyle.Render(title))
	}
	return result
}
