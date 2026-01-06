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

const NUM_COLUMNS = 2

type installedInitMsg struct{} // Indicate tab setup I/O

type installedModel struct {
	hotkeyViewport viewport.Model
	listViewport   viewport.Model
	infoViewport   viewport.Model
	searchInput    textinput.Model

	packageLines        []string
	visiblePackageLines []int
	infoLines           []string

	fullHeight int
	listCursor int
	listCmdId  int
	infoCmdId  int

	hasViewportDimensions      bool
	isViewingHotkeyPanel       bool
	isFinishedReadingLines     bool
	isFilteringExplicitInstall bool

	cmds []tea.Cmd

	hotkeys map[string]hotkeyBinding

	startRoutes map[StreamTarget]handler[*installedModel, CommandStartMsg]
	chunkRoutes map[StreamTarget]handler[*installedModel, CommandChunkMsg]
	doneRoutes  map[StreamTarget]handler[*installedModel, CommandDoneMsg]
}

func initialInstalledModel() *installedModel {
	model := installedModel{
		packageLines:        make([]string, 0, 2048),
		visiblePackageLines: make([]int, 0, 2048),
		infoLines:           make([]string, 0, 100),

		listCursor:             0,
		hasViewportDimensions:  false,
		isFinishedReadingLines: false,

		cmds:    make([]tea.Cmd, 0, 6),
		hotkeys: make(map[string]hotkeyBinding),

		startRoutes: map[StreamTarget]handler[*installedModel, CommandStartMsg]{
			PackageList: func(m *installedModel, msg CommandStartMsg) tea.Cmd {
				m.isFinishedReadingLines = false
				m.listCmdId = msg.CommandId

				m.packageLines = m.packageLines[:0]
				m.visiblePackageLines = m.visiblePackageLines[:0]

				m.listViewport.SetContent("Loading installed packages...")
				return nil
			},
			PackageInfo: func(m *installedModel, msg CommandStartMsg) tea.Cmd {
				m.infoCmdId = msg.CommandId
				m.infoLines = m.infoLines[:0]
				return nil
			},
		},

		chunkRoutes: map[StreamTarget]handler[*installedModel, CommandChunkMsg]{
			PackageList: func(m *installedModel, msg CommandChunkMsg) tea.Cmd {
				if msg.CommandId != m.listCmdId {
					return nil
				}

				m.packageLines = append(m.packageLines, msg.Lines...)
				m.buildPackageList()
				return nil
			},
			PackageInfo: func(m *installedModel, msg CommandChunkMsg) tea.Cmd {
				if msg.CommandId != m.infoCmdId {
					return nil
				}

				m.infoLines = append(m.infoLines, msg.Lines...)
				m.buildInfoList()
				return nil
			},
		},

		doneRoutes: map[StreamTarget]handler[*installedModel, CommandDoneMsg]{
			PackageList: func(m *installedModel, msg CommandDoneMsg) tea.Cmd {
				if msg.CommandId == m.listCmdId && msg.Err != nil {
					m.packageLines = append(m.packageLines, fmt.Sprintf("\n%s\n", msg.Err))
				}

				m.isFinishedReadingLines = true

				if !m.searchInput.Focused() && len(m.visiblePackageLines) > 0 {
					m.cmds = append(m.cmds, getPackageInfo(m.packageLines[m.visiblePackageLines[0]]))
				}
				return nil
			},
			PackageInfo: func(m *installedModel, msg CommandDoneMsg) tea.Cmd {
				if msg.CommandId == m.infoCmdId && msg.Err != nil {
					m.infoLines = append(m.infoLines, fmt.Sprintf("\n%s\n", msg.Err))
					m.buildInfoList()
				}
				return nil
			},
		},
	}

	model.hotkeys["enter"] = hotkeyBinding{"Enter", "Toggle Search", model.toggleSearch}
	model.hotkeys["A"] = hotkeyBinding{"A", "Upgrade All", model.upgradeAll}
	model.hotkeys["D"] = hotkeyBinding{"D", "Remove Selected", model.removeSelected}
	model.hotkeys["E"] = hotkeyBinding{"E", "Show/Hide Explicitly Installed", model.toggleExplicitFilter}
	model.hotkeys["H"] = hotkeyBinding{"H", "Show/Hide Hotkeys", model.toggleHotkeyPanel}
	model.hotkeys["R"] = hotkeyBinding{"R", "Refresh List", model.getInstalledPackages}
	model.hotkeys["U"] = hotkeyBinding{"U", "Upgrade Selected", model.upgradeSelected}

	return &model
}

func (m *installedModel) Init() tea.Cmd {
	return func() tea.Msg { return installedInitMsg{} }
}

func (m *installedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.cmds = m.cmds[:0]

	switch msg := msg.(type) {

	case installedInitMsg:
		m.cmds = append(m.cmds, getInstalledPackages())

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
		m.fullHeight = msg.Height
		lw := int(float32(msg.Width) * float32(0.4))
		rw := msg.Width - lw - 1
		if !m.hasViewportDimensions {
			m.listViewport = viewport.New(lw, msg.Height-1)
			m.hotkeyViewport = viewport.New(lw, len(m.hotkeys))
			m.infoViewport = viewport.New(rw, msg.Height)

			m.searchInput = textinput.New()
			m.searchInput.Width = lw - 4 // Subtract cursor and a space

			m.hasViewportDimensions = true
		} else {
			m.hotkeyViewport.Width = lw
			m.hotkeyViewport.Height = len(m.hotkeys)

			m.listViewport.Width = lw

			if m.isViewingHotkeyPanel {
				m.listViewport.Height = msg.Height - len(m.hotkeys) - 1
			} else {
				m.listViewport.Height = msg.Height - 1
			}
			m.searchInput.Width = rw - 2

			m.infoViewport.Width = rw
			m.infoViewport.Height = msg.Height

		}

	case tea.KeyMsg:
		val, hotkeyExists := m.hotkeys[msg.String()]
		if hotkeyExists {
			m.cmds = append(m.cmds, func() tea.Msg { return HotkeyPressedMsg{hotkey: val} })
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
				m.listCursor = 0
				m.buildPackageList()

				if len(m.visiblePackageLines) > 0 {
					m.cmds = append(m.cmds, getPackageInfo(m.packageLines[m.visiblePackageLines[0]]))
				} else {
					m.infoLines = m.infoLines[:0]
					m.infoViewport.SetContent("")
				}
			}
		} else {
			switch msg.String() {
			case "up", "k":
				if m.listCursor > 0 {
					m.listCursor--

					m.buildPackageList()
					pkg := m.packageLines[m.visiblePackageLines[m.listCursor]]
					m.cmds = append(m.cmds, getPackageInfo(pkg))

					if m.listCursor < m.listViewport.YOffset {
						updated, cmd := m.listViewport.Update(msg)
						m.listViewport = updated
						m.cmds = append(m.cmds, cmd)
					}
				}

			case "down", "j":
				if m.listCursor < (len(m.visiblePackageLines) - 1) {
					m.listCursor++

					m.buildPackageList()
					pkg := m.packageLines[m.visiblePackageLines[m.listCursor]]
					m.cmds = append(m.cmds, getPackageInfo(pkg))

					if m.listCursor >= m.listViewport.YOffset+m.listViewport.VisibleLineCount() {
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

func (m *installedModel) View() string {

	if !m.hasViewportDimensions {
		return "Initialising..."
	}

	var viewMode string
	if m.isFilteringExplicitInstall {
		viewMode = "Explicit"
	} else {
		viewMode = "All"
	}
	counterText := reducedEmphasisStyle.Render(fmt.Sprintf("%d / %d (%s)", m.listCursor+1, len(m.visiblePackageLines), viewMode))
	listViewport := m.listViewport.View()

	topRow := lipgloss.PlaceHorizontal(m.listViewport.Width-2, lipgloss.Right, counterText)
	if m.searchInput.Focused() {
		topRow = defaultStyle.Render(m.searchInput.View())
		listViewport = reducedEmphasisStyle.Render(listViewport)
	}

	n := max(1, len(m.visiblePackageLines))
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
			float64(m.listCursor) *
				float64(lipgloss.Height(listViewport)) /
				float64(len(m.visiblePackageLines)))

		scrollBarString = defaultStyle.PaddingTop(int(yRelative)).Render(scrollBarString)
	}

	listViewport = lipgloss.JoinHorizontal(lipgloss.Left, listViewport, scrollBarString)
	listPanel := lipgloss.JoinVertical(lipgloss.Left, topRow, listViewport)

	leftCol := panelStyle.Render(listPanel)
	if m.isViewingHotkeyPanel {
		leftCol = lipgloss.JoinVertical(
			lipgloss.Left,
			panelStyle.Render(listPanel),
			panelStyle.Render(m.hotkeyViewport.View()),
		)
	}

	infoViewport := panelStyle.Render(m.infoViewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Left, leftCol, infoViewport)
}

func (m *installedModel) getInstalledPackages() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	return getInstalledPackages()
}

func (m *installedModel) upgradeAll() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	return upgradeAll()
}

func (m *installedModel) upgradeSelected() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	selectedLine := m.packageLines[m.visiblePackageLines[m.listCursor]]
	return upgradeSelected(selectedLine)
}

func (m *installedModel) removeSelected() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	selectedLine := m.packageLines[m.visiblePackageLines[m.listCursor]]
	return removeSelected(selectedLine)
}

func (m *installedModel) toggleExplicitFilter() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	isFiltering := !m.isFilteringExplicitInstall
	m.isFilteringExplicitInstall = isFiltering

	if isFiltering {
		return getExplicitlyInstalledPackages()
	} else {
		return getInstalledPackages()
	}
}

// TODO make sure we scroll to the selected package if showing the hotkey panel obscured it
func (m *installedModel) toggleHotkeyPanel() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	m.isViewingHotkeyPanel = !m.isViewingHotkeyPanel
	if m.isViewingHotkeyPanel {
		m.listViewport.Height = m.listViewport.Height - m.hotkeyViewport.Height - 2
	} else {
		m.listViewport.Height = m.fullHeight - 1
	}

	var list strings.Builder
	for _, shortcut := range m.hotkeys {
		list.WriteString(
			fmt.Sprintf(
				"%s%s%s\n",
				hotkeyStyle.Render(shortcut.shortcut),
				strings.Repeat(" ", m.hotkeyViewport.Width-len(shortcut.shortcut)-len(shortcut.description)-1),
				reducedEmphasisStyle.Render(shortcut.description),
			),
		)
	}

	m.hotkeyViewport.SetContent(list.String())
	return nil
}

func (m *installedModel) toggleSearch() tea.Cmd {
	if m.searchInput.Focused() {
		m.searchInput.Blur()
	} else {
		m.searchInput.Focus()
		m.searchInput.Width = 10
	}

	return nil
}

func (m *installedModel) buildPackageList() {
	m.visiblePackageLines = m.visiblePackageLines[:0]
	searchText := m.searchInput.Value()
	for i, line := range m.packageLines {
		if matchesSearch(line, searchText) {
			m.visiblePackageLines = append(m.visiblePackageLines, i)
		}
	}

	if m.listCursor >= len(m.visiblePackageLines) {
		m.listCursor = 0
	}

	var builder strings.Builder
	for i, lineIdx := range m.visiblePackageLines {
		name, _, _ := strings.Cut(m.packageLines[lineIdx], "\n")
		if m.listCursor == i {
			builder.WriteString(selectedStyle.Render(name) + "\n")
		} else {
			builder.WriteString(m.packageLines[lineIdx])
		}
	}

	m.listViewport.SetContent(builder.String())
}

func (m *installedModel) buildInfoList() {
	var builder strings.Builder
	for _, line := range m.infoLines {
		builder.WriteString(line)
	}

	m.infoViewport.SetContent(builder.String())
}
