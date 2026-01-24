package main

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	cmd "ptui/command"
	"ptui/types"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type browseModel struct {
	listViewport   viewport.Model
	infoViewport   viewport.Model
	hotkeyViewport viewport.Model
	searchInput    textinput.Model

	searchResultLines        []string
	visibleSearchResultLines []int

	infoLines []string

	fullHeight         int
	searchResultCursor int

	infoCmdId int
	listCmdId int

	hasViewportDimensions  bool
	isFinishedReadingLines bool
	isViewingList          bool
	isViewingHotkeys       bool

	hotkeys        map[string]types.HotkeyBinding
	hotkeysOrdered []string

	startRoutes types.MessageRouter[*browseModel, cmd.CommandStartMsg]
	chunkRoutes types.MessageRouter[*browseModel, cmd.CommandChunkMsg]
	doneRoutes  types.MessageRouter[*browseModel, cmd.CommandDoneMsg]

	cmds []tea.Cmd
}

type browseInitMsg struct{}

func initialBrowseModel() *browseModel {
	model := browseModel{
		searchResultCursor: 0,
		isViewingList:      true,
		hotkeys:            make(map[string]types.HotkeyBinding),

		startRoutes: types.MessageRouter[*browseModel, cmd.CommandStartMsg]{
			PackageList: func(m *browseModel, msg cmd.CommandStartMsg) tea.Cmd {
				m.isFinishedReadingLines = false
				m.listCmdId = msg.CommandId
				m.searchResultLines = m.searchResultLines[:0]
				m.visibleSearchResultLines = m.visibleSearchResultLines[:0]

				m.listViewport.SetContent("Loading results...")
				return nil
			},
			PackageInfo: func(m *browseModel, msg cmd.CommandStartMsg) tea.Cmd {
				m.infoCmdId = msg.CommandId
				m.infoLines = m.infoLines[:0]
				return nil
			},
		},
		chunkRoutes: types.MessageRouter[*browseModel, cmd.CommandChunkMsg]{
			PackageList: func(m *browseModel, msg cmd.CommandChunkMsg) tea.Cmd {
				if m.listCmdId != msg.CommandId {
					return nil
				}

				m.searchResultLines = append(m.searchResultLines, msg.Lines...)
				m.buildPackageList()
				return nil
			},
			PackageInfo: func(m *browseModel, msg cmd.CommandChunkMsg) tea.Cmd {
				if msg.CommandId != m.infoCmdId {
					return nil
				}

				m.infoLines = append(m.infoLines, msg.Lines...)
				m.buildInfoList()
				return nil
			},
		},
		doneRoutes: types.MessageRouter[*browseModel, cmd.CommandDoneMsg]{
			PackageList: func(m *browseModel, msg cmd.CommandDoneMsg) tea.Cmd {
				if m.listCmdId != msg.CommandId {
					return nil
				}

				if msg.Err != nil {
					m.searchResultLines = append(m.searchResultLines, fmt.Sprintf("\n%s\n", msg.Err))
					m.buildPackageList()
				}

				m.isFinishedReadingLines = true

				return nil
			},
			PackageInfo: func(m *browseModel, msg cmd.CommandDoneMsg) tea.Cmd {
				if msg.CommandId == m.infoCmdId && msg.Err != nil {
					m.infoLines = append(m.infoLines, fmt.Sprintf("\n%s\n", msg.Err))
					m.buildInfoList()
				}
				return nil
			},
		},
	}

	model.createHotkey("H", "H", "Toggle Hotkeys", model.ToggleHotkeys)
	model.createHotkey("/", "/", "Toggle Search", model.toggleSearch)
	model.createHotkey("I", "I", "View Details", model.viewDetails)
	model.createHotkey("backspace", "Backspace", "Close Details", model.closeDetails)
	model.createHotkey("enter", "Enter", "Install Selected", model.installSelected)

	slices.SortFunc(model.hotkeysOrdered, func(a, b string) int {
		hotkeyA := model.hotkeys[a]
		hotkeyB := model.hotkeys[b]

		return cmp.Compare(hotkeyA.Description, hotkeyB.Description)
	})

	return &model
}

func (m *browseModel) createHotkey(key string, displayKey string, description string, action func() tea.Cmd) {
	hotkey := types.HotkeyBinding{Shortcut: displayKey, Description: description, Command: action}

	m.hotkeys[key] = hotkey
	m.hotkeysOrdered = append(m.hotkeysOrdered, key)
}

func (m *browseModel) Init() tea.Cmd {
	return func() tea.Msg { return browseInitMsg{} }
}

func (m *browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.cmds = m.cmds[:0]

	switch msg := msg.(type) {
	case browseInitMsg:
		m.cmds = append(m.cmds, m.searchPackageDatabase(""))

	case cmd.CommandStartMsg:
		handler, exists := m.startRoutes[msg.Target]
		if exists {
			handler(m, msg)
		}

	case cmd.CommandChunkMsg:
		handler, exists := m.chunkRoutes[msg.Target]
		if exists {
			handler(m, msg)
		}

	case cmd.CommandDoneMsg:
		handler, exists := m.doneRoutes[msg.Target]
		if exists {
			handler(m, msg)
		}

	case types.ContentRectMsg:
		ch := msg.Height - 2
		if m.hasViewportDimensions {
			m.fullHeight = msg.Height

			m.listViewport.Height = ch
			m.listViewport.Width = msg.Width

			m.hotkeyViewport.Width = msg.Width
			m.hotkeyViewport.Height = len(m.hotkeys)

			m.infoViewport.Height = ch
			m.infoViewport.Width = msg.Width

			m.searchInput.Width = msg.Width
		} else {
			m.listViewport = viewport.New(msg.Width, ch)
			m.infoViewport = viewport.New(msg.Width, ch)
			m.hotkeyViewport = viewport.New(msg.Width, len(m.hotkeys))

			m.searchInput = textinput.New()
			m.searchInput.Width = msg.Width

			m.hasViewportDimensions = true
		}
	case tea.KeyMsg:
		handleHotkeyAndSearch(m, msg)

		switch msg.String() {
		case "up", "k":
			if m.searchResultCursor > 0 {
				m.searchResultCursor--
				m.buildPackageList()
				scrollIntoView(&m.listViewport, m.searchResultCursor)

			}
		case "down", "j":
			if m.searchResultCursor < (len(m.visibleSearchResultLines) - 1) {
				m.searchResultCursor++
				m.buildPackageList()
				scrollIntoView(&m.listViewport, m.searchResultCursor)
			}
		}
	}

	return m, tea.Batch(m.cmds...)
}

func (m *browseModel) View() (final string) {
	var topRow string
	if m.searchInput.Focused() {
		topRow = m.searchInput.View()
	}

	var viewport string
	if m.isViewingList {
		viewport = m.listViewport.View()
	} else {
		viewport = m.infoViewport.View()
	}

	if m.searchInput.Focused() {
		viewport = reducedEmphasisStyle.Render(viewport)
	}

	var hotKeyPanel string
	if m.isViewingHotkeys {
		hotKeyPanel = panelStyle.Render(m.hotkeyViewport.View())
	}

	scrollbar := createScrollbar(
		2,
		m.searchResultCursor,
		len(m.visibleSearchResultLines),
		lipgloss.Height(viewport),
		m.isFinishedReadingLines,
	)

	mainPanel := lipgloss.JoinHorizontal(lipgloss.Left, viewport, scrollbar)
	mainPanel = lipgloss.JoinVertical(lipgloss.Left, mainPanel, hotKeyPanel)

	final = lipgloss.JoinVertical(lipgloss.Left, topRow, mainPanel)

	return final
}

func (m *browseModel) StatusBar() string {
	counterText := fmt.Sprintf("%d / %d", m.searchResultCursor+1, len(m.visibleSearchResultLines))
	return counterText
}

func (m *browseModel) ToggleHotkeys() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	m.isViewingHotkeys = !m.isViewingHotkeys
	if m.isViewingHotkeys {
		m.listViewport.Height = m.listViewport.Height - m.hotkeyViewport.Height
	} else {
		m.listViewport.Height += m.hotkeyViewport.Height
	}

	buildSortedHotkeyList(&m.hotkeyViewport, m.hotkeys, m.hotkeysOrdered)
	scrollIntoView(&m.listViewport, m.searchResultCursor)

	return nil
}

func (m *browseModel) toggleSearch() tea.Cmd {
	if !m.isViewingList {
		return nil
	}

	if m.searchInput.Focused() {
		m.searchInput.Blur()
	} else {
		m.searchInput.Focus()
		m.searchInput.Width = 10
	}

	return nil
}

func (m *browseModel) buildPackageList() {
	m.visibleSearchResultLines = m.visibleSearchResultLines[:0]

	searchText := m.searchInput.Value()
	for i, line := range m.searchResultLines {
		if matchesSearch(line, searchText) {
			m.visibleSearchResultLines = append(m.visibleSearchResultLines, i)
		}
	}

	if m.searchResultCursor >= len(m.visibleSearchResultLines) {
		m.searchResultCursor = 0
	}

	var builder strings.Builder
	for i, lineIdx := range m.visibleSearchResultLines {
		name, _, _ := strings.Cut(m.searchResultLines[lineIdx], "\n")
		if i == m.searchResultCursor {
			builder.WriteString(selectedStyle.Render(name) + "\n")
		} else {
			builder.WriteString(m.searchResultLines[lineIdx])
		}
	}

	m.listViewport.SetContent(builder.String())
}

func (m *browseModel) buildInfoList() {
	var builder strings.Builder
	for _, line := range m.infoLines {
		builder.WriteString(line)
	}

	m.infoViewport.SetContent(builder.String())
}

func (m *browseModel) viewDetails() tea.Cmd {
	m.isViewingList = false
	m.searchInput.Blur()

	name, err := m.getSelectedPackageName()
	if err != nil {
		return nil
	}

	return cmd.NewCommand().
		Operation("S").
		Options("i").
		Arguments(name, "--noconfirm").
		Target(PackageInfo).
		Run()
}

func (m *browseModel) closeDetails() tea.Cmd {
	m.isViewingList = true
	m.infoViewport.SetContent("")

	return nil
}

func (m *browseModel) Hotkeys() map[string]types.HotkeyBinding {
	return m.hotkeys
}

func (m *browseModel) SearchInput() *textinput.Model {
	return &m.searchInput
}

func (m *browseModel) AddCommand(cmd tea.Cmd) {
	m.cmds = append(m.cmds, cmd)
}

func (m *browseModel) ResetCursor() {
	m.searchResultCursor = 0
	m.buildPackageList()
}

func (m *browseModel) searchPackageDatabase(text string) tea.Cmd {
	return cmd.NewCommand().
		Operation("S").
		Options("s", "q").
		Arguments(text, "--noconfirm").
		Target(PackageList).
		Run()
}

func (m *browseModel) installSelected() tea.Cmd {
	name, err := m.getSelectedPackageName()

	if err != nil {
		return nil
	}

	return cmd.NewCommand().
		Operation("S").
		Arguments(name, "--noconfirm").
		Target(Background).
		Run()
}

func (m *browseModel) getSelectedPackageName() (string, error) {
	if len(m.visibleSearchResultLines) == 0 {
		return "", errors.New("No packages in list")
	}

	return strings.TrimSuffix(m.searchResultLines[m.visibleSearchResultLines[m.searchResultCursor]], "\n"), nil
}
