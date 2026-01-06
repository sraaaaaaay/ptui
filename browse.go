package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type browseModel struct {
	listViewport viewport.Model
	searchInput  textinput.Model

	searchResultLines        []string
	visibleSearchResultLines []int

	fullHeight         int
	searchResultCursor int
	listCmdId          int

	hasViewportDimensions  bool
	isFinishedReadingLines bool

	hotkeys map[string]hotkeyBinding

	startRoutes map[StreamTarget]handler[*browseModel, CommandStartMsg]
	chunkRoutes map[StreamTarget]handler[*browseModel, CommandChunkMsg]
	doneRoutes  map[StreamTarget]handler[*browseModel, CommandDoneMsg]

	cmds []tea.Cmd
}

type browseInitMsg struct{}

func initialBrowseModel() *browseModel {
	model := browseModel{
		searchResultCursor: 0,
		hotkeys:            make(map[string]hotkeyBinding),
		startRoutes: map[StreamTarget]handler[*browseModel, CommandStartMsg]{
			SearchResultList: func(m *browseModel, msg CommandStartMsg) tea.Cmd {
				m.isFinishedReadingLines = false
				m.listCmdId = msg.CommandId
				m.searchResultLines = m.searchResultLines[:0]
				m.visibleSearchResultLines = m.visibleSearchResultLines[:0]

				m.listViewport.SetContent("Loading results...")
				return nil
			},
		},
		chunkRoutes: map[StreamTarget]handler[*browseModel, CommandChunkMsg]{
			SearchResultList: func(m *browseModel, msg CommandChunkMsg) tea.Cmd {
				if m.listCmdId != msg.CommandId {
					return nil
				}

				m.searchResultLines = append(m.searchResultLines, msg.Lines...)
				m.buildPackageList()
				return nil
			},
		},
		doneRoutes: map[StreamTarget]handler[*browseModel, CommandDoneMsg]{
			SearchResultList: func(m *browseModel, msg CommandDoneMsg) tea.Cmd {
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
		},
	}

	model.hotkeys["enter"] = hotkeyBinding{"Enter", "Toggle Search", model.toggleSearch}

	return &model
}

func (m *browseModel) Init() tea.Cmd {
	return func() tea.Msg { return browseInitMsg{} }
}

func (m *browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.cmds = m.cmds[:0]

	switch msg := msg.(type) {
	case browseInitMsg:
		m.cmds = append(m.cmds, searchPackageDatabase(""))

	case CommandStartMsg:
		handler, exists := m.startRoutes[msg.Target]
		if exists {
			handler(m, msg)
		}

	case CommandChunkMsg:
		handler, exists := m.chunkRoutes[msg.Target]
		if exists {
			handler(m, msg)
		}

	case CommandDoneMsg:
		handler, exists := m.doneRoutes[msg.Target]
		if exists {
			handler(m, msg)
		}

	case ContentRectMsg:
		if m.hasViewportDimensions {
			m.fullHeight = msg.Height

			m.listViewport.Height = msg.Height
			m.listViewport.Width = msg.Width

			m.searchInput.Width = msg.Width - 4
		} else {
			m.listViewport = viewport.New(msg.Width, msg.Height)

			m.searchInput = textinput.New()
			m.searchInput.Width = msg.Width - 4

			m.hasViewportDimensions = true
		}
	case tea.KeyMsg:
		hotkey, exists := m.hotkeys[msg.String()]
		if exists {
			m.cmds = append(m.cmds, func() tea.Msg { return HotkeyPressedMsg{hotkey: hotkey} })
		}

		if m.searchInput.Focused() {
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

	listViewport := m.listViewport.View()

	n := max(1, len(m.visibleSearchResultLines))
	size := max(1, int(math.Round(float64(lipgloss.Height(listViewport))/float64(n))))

	var scrollBar strings.Builder
	for i := range size {
		if i < size-1 {
			scrollBar.WriteString("█\n")
		} else {
			scrollBar.WriteRune('█')
		}
	}
	scrollBarString := scrollBar.String()

	if m.isFinishedReadingLines {
		yRelative := math.Round(
			float64(m.searchResultCursor) *
				float64(lipgloss.Height(listViewport)) /
				float64(len(m.visibleSearchResultLines)))

		scrollBarString = defaultStyle.PaddingTop(int(yRelative)).Render(scrollBarString)
	}

	mainPanel := lipgloss.JoinHorizontal(lipgloss.Left, listViewport, scrollBarString)
	final = lipgloss.JoinVertical(lipgloss.Left, topRow, mainPanel)

	return panelStyle.Render(final)
}

func (m *browseModel) toggleSearch() tea.Cmd {
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
