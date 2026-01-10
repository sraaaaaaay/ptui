package main

import (
	"errors"
	"fmt"
	"strings"

	cmd "ptui/command"
	"ptui/types"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type browseModel struct {
	listViewport viewport.Model
	infoViewport viewport.Model
	searchInput  textinput.Model

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

	hotkeys map[string]types.HotkeyBinding

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
			SearchResultList: func(m *browseModel, msg cmd.CommandStartMsg) tea.Cmd {
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
			SearchResultList: func(m *browseModel, msg cmd.CommandChunkMsg) tea.Cmd {
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
			SearchResultList: func(m *browseModel, msg cmd.CommandDoneMsg) tea.Cmd {
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

	model.createHotkey("/", "/", "Toggle Search", model.toggleSearch)
	model.createHotkey("enter", "Enter", "View Details", model.viewDetails)
	model.createHotkey("esc", "Esc", "Close Details", model.closeDetails)

	return &model
}

func (m *browseModel) createHotkey(key string, displayKey string, description string, action func() tea.Cmd) {
	m.hotkeys[key] = types.HotkeyBinding{Shortcut: displayKey, Description: description, Command: action}
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
		cw := msg.Width
		ch := msg.Height - 2
		if m.hasViewportDimensions {
			m.fullHeight = msg.Height

			m.listViewport.Height = ch
			m.listViewport.Width = msg.Width

			m.infoViewport.Height = ch
			m.infoViewport.Width = msg.Width

			m.searchInput.Width = cw
		} else {
			m.listViewport = viewport.New(msg.Width, ch)
			m.infoViewport = viewport.New(msg.Width, ch)

			m.searchInput = textinput.New()
			m.searchInput.Width = cw

			m.hasViewportDimensions = true
		}
	case tea.KeyMsg:
		hotkey, exists := m.hotkeys[msg.String()]

		if exists {
			m.cmds = append(m.cmds, func() tea.Msg { return types.HotkeyPressedMsg{Hotkey: hotkey} })
		}

		if msg.String() != "/" && m.searchInput.Focused() {
			oldVal := m.searchInput.Value()
			updated, cmd := m.searchInput.Update(msg)
			newVal := updated.Value()

			m.searchInput = updated
			if cmd != nil {
				m.cmds = append(m.cmds, cmd)
			}

			if oldVal != newVal {
				m.searchResultCursor = 0
				m.buildPackageList()
			}

		} else {
			switch msg.String() {
			case "up", "k":
				if m.searchResultCursor > 0 {
					m.searchResultCursor--
					m.buildPackageList()

					if m.searchResultCursor < m.listViewport.YOffset {
						updated, cmd := m.listViewport.Update(msg)
						m.listViewport = updated
						m.cmds = append(m.cmds, cmd)
					}
				}
			case "down", "j":
				if m.searchResultCursor < (len(m.visibleSearchResultLines) - 1) {
					m.searchResultCursor++
					m.buildPackageList()

					if m.searchResultCursor >= m.listViewport.YOffset+m.listViewport.VisibleLineCount() {
						updated, cmd := m.listViewport.Update(msg)
						m.listViewport = updated
						m.cmds = append(m.cmds, cmd)
					}
				}
			}
		}
	}

	return m, tea.Batch(m.cmds...)
}

func (m *browseModel) View() (final string) {
	var topRow string
	if m.searchInput.Focused() {
		topRow = m.searchInput.View()
	} else {
		topRow = lipgloss.PlaceHorizontal(
			m.listViewport.Width-2,
			lipgloss.Right,
			reducedEmphasisStyle.Render(
				fmt.Sprintf("%d / %d", m.searchResultCursor+1, len(m.visibleSearchResultLines))))
	}

	var viewport string
	if m.isViewingList {
		viewport = m.listViewport.View()
	} else {
		viewport = m.infoViewport.View()
	}

	scrollBarString := createScrollbar(
		2,
		m.searchResultCursor,
		len(m.visibleSearchResultLines),
		lipgloss.Height(viewport),
		m.isFinishedReadingLines,
	)

	mainPanel := lipgloss.JoinHorizontal(lipgloss.Left, viewport, scrollBarString)
	final = lipgloss.JoinVertical(lipgloss.Left, topRow, mainPanel)

	return final
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
	return nil
}

func (m *browseModel) searchPackageDatabase(text string) tea.Cmd {
	return cmd.NewCommand().
		Operation("S").
		Options("s", "q").
		Arguments(text, "--noconfirm").
		Target(SearchResultList).
		Run()
}

func (m *browseModel) getSelectedPackageName() (string, error) {
	if len(m.visibleSearchResultLines) == 0 {
		return "", errors.New("No packages in list")
	}

	return strings.TrimSuffix(m.searchResultLines[m.visibleSearchResultLines[m.searchResultCursor]], "\n"), nil
}
