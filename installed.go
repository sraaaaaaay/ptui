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

	hotkeys        map[string]types.HotkeyBinding
	hotkeysOrdered []string

	startRoutes types.MessageRouter[*installedModel, cmd.CommandStartMsg]
	chunkRoutes types.MessageRouter[*installedModel, cmd.CommandChunkMsg]
	doneRoutes  types.MessageRouter[*installedModel, cmd.CommandDoneMsg]
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
		hotkeys: make(map[string]types.HotkeyBinding),

		startRoutes: types.MessageRouter[*installedModel, cmd.CommandStartMsg]{
			PackageList: func(m *installedModel, msg cmd.CommandStartMsg) tea.Cmd {
				m.isFinishedReadingLines = false
				m.listCmdId = msg.CommandId

				m.packageLines = m.packageLines[:0]
				m.visiblePackageLines = m.visiblePackageLines[:0]

				m.listViewport.SetContent("Loading installed packages...")
				return nil
			},
			PackageInfo: func(m *installedModel, msg cmd.CommandStartMsg) tea.Cmd {
				m.infoCmdId = msg.CommandId
				m.infoLines = m.infoLines[:0]
				return nil
			},
		},

		chunkRoutes: types.MessageRouter[*installedModel, cmd.CommandChunkMsg]{
			PackageList: func(m *installedModel, msg cmd.CommandChunkMsg) tea.Cmd {
				if msg.CommandId != m.listCmdId {
					return nil
				}

				m.packageLines = append(m.packageLines, msg.Lines...)
				m.buildPackageList()
				return nil
			},
			PackageInfo: func(m *installedModel, msg cmd.CommandChunkMsg) tea.Cmd {
				if msg.CommandId != m.infoCmdId {
					return nil
				}

				m.infoLines = append(m.infoLines, msg.Lines...)
				m.buildInfoList()
				return nil
			},
		},

		doneRoutes: types.MessageRouter[*installedModel, cmd.CommandDoneMsg]{
			PackageList: func(m *installedModel, msg cmd.CommandDoneMsg) tea.Cmd {
				if msg.CommandId == m.listCmdId && msg.Err != nil {
					m.packageLines = append(m.packageLines, fmt.Sprintf("\n%s\n", msg.Err))
				}

				m.isFinishedReadingLines = true

				if !m.searchInput.Focused() && len(m.visiblePackageLines) > 0 {
					m.cmds = append(m.cmds, m.getPackageInfo())
				}
				return nil
			},
			PackageInfo: func(m *installedModel, msg cmd.CommandDoneMsg) tea.Cmd {
				if msg.CommandId == m.infoCmdId && msg.Err != nil {
					m.infoLines = append(m.infoLines, fmt.Sprintf("\n%s\n", msg.Err))
					m.buildInfoList()
				}
				return nil
			},
		},
	}

	model.createHotkey("/", "/", "Toggle Search", model.toggleSearch)
	model.createHotkey("A", "A", "Upgrade All", model.upgradeAll)
	model.createHotkey("R", "R", "Remove Selected", model.removeSelected)
	model.createHotkey("E", "E", "Toggle Explicit", model.toggleExplicitFilter)
	model.createHotkey("H", "H", "Toggle Hotkeys", model.toggleHotkeys)
	model.createHotkey("U", "U", "Upgrade Selected", model.upgradeSelected)

	slices.SortFunc(model.hotkeysOrdered, func(a, b string) int {
		hotkeyA := model.hotkeys[a]
		hotkeyB := model.hotkeys[b]

		return cmp.Compare(hotkeyA.Description, hotkeyB.Description)
	})

	return &model
}

func (m *installedModel) createHotkey(key string, displayKey string, description string, action func() tea.Cmd) {
	m.hotkeys[key] = types.HotkeyBinding{Shortcut: displayKey, Description: description, Command: action}
	m.hotkeysOrdered = append(m.hotkeysOrdered, key)
}

func (m *installedModel) Init() tea.Cmd {
	return func() tea.Msg { return installedInitMsg{} }
}

func (m *installedModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.cmds = m.cmds[:0]

	switch msg := msg.(type) {

	case installedInitMsg:
		m.cmds = append(m.cmds, m.getInstalledPackages())

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
		m.fullHeight = msg.Height
		lw := int(float32(msg.Width) * float32(0.4))
		rw := msg.Width - lw - 1
		if !m.hasViewportDimensions {
			m.listViewport = viewport.New(lw, msg.Height-1)
			m.hotkeyViewport = viewport.New(lw+1, len(m.hotkeys))
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

			m.buildInfoList()
		}

	case tea.KeyMsg:
		val, hotkeyExists := m.hotkeys[msg.String()]
		if hotkeyExists {
			m.cmds = append(m.cmds, func() tea.Msg { return types.HotkeyPressedMsg{Hotkey: val} })
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
					m.cmds = append(m.cmds, m.getPackageInfo())
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
					m.cmds = append(m.cmds, m.getPackageInfo())
					scrollIntoView(&m.listViewport, m.listCursor)
				}

			case "down", "j":
				if m.listCursor < (len(m.visiblePackageLines) - 1) {
					m.listCursor++

					m.buildPackageList()
					m.cmds = append(m.cmds, m.getPackageInfo())
					scrollIntoView(&m.listViewport, m.listCursor)
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

	listViewport := m.listViewport.View()

	var topRow string
	if m.searchInput.Focused() {
		topRow = defaultStyle.Render(m.searchInput.View())
		listViewport = reducedEmphasisStyle.Render(listViewport)
	}

	scrollBarString := createScrollbar(
		1,
		m.listCursor,
		len(m.visiblePackageLines),
		lipgloss.Height(listViewport),
		m.isFinishedReadingLines,
	)

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

func (m *installedModel) StatusBar() string {
	var viewMode string
	if m.isFilteringExplicitInstall {
		viewMode = "Explicit"
	} else {
		viewMode = "All"
	}

	counterText := fmt.Sprintf(" %d / %d", m.listCursor+1, len(m.visiblePackageLines))

	listPanelEdge := m.listViewport.Width - len(counterText) + 2
	viewMode = lipgloss.PlaceHorizontal(listPanelEdge, lipgloss.Right, viewMode)

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		counterText,
		viewMode)
}

func (m *installedModel) toggleExplicitFilter() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	m.isFilteringExplicitInstall = !m.isFilteringExplicitInstall

	return m.getInstalledPackages()
}

func (m *installedModel) toggleHotkeys() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	m.isViewingHotkeyPanel = !m.isViewingHotkeyPanel
	if m.isViewingHotkeyPanel {
		m.listViewport.Height = m.listViewport.Height - m.hotkeyViewport.Height - 2
	} else {
		m.listViewport.Height = m.fullHeight - 1
	}

	buildSortedHotkeyList(&m.hotkeyViewport, m.hotkeys, m.hotkeysOrdered)
	scrollIntoView(&m.listViewport, m.listCursor)

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
	maxWidth := m.infoViewport.Width

	leftColWidth := 18
	rightColWidth := maxWidth - leftColWidth
	rightStyle := defaultStyle.Width(rightColWidth)

	if rightColWidth <= 0 {
		return
	}

	for _, line := range m.infoLines {
		// Lipgloss's auto-wrapping destroys URLs for most terminals.
		// Prefer to make the line hard-to-read than useless.

		// TODO fix if a URL runs off the terminal, the unrendered characters are treated as junk anyway and can't be clicked.
		if isUrl(line) {
			builder.WriteString(line)
			continue
		}

		halves := strings.Split(line, " : ")
		if len(halves) != 2 {
			builder.WriteString(line)
			continue
		}

		key := halves[0]
		value := strings.TrimSpace(halves[1])

		valueRendered := rightStyle.Render(value)
		row := lipgloss.JoinHorizontal(lipgloss.Top, key, valueRendered)
		builder.WriteString(row + "\n")
	}

	m.infoViewport.SetContent(builder.String())
}

func (m *installedModel) getInstalledPackages() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	cmd := cmd.NewCommand().
		Operation("Q").
		Options("q").
		Target(PackageList)

	if m.isFilteringExplicitInstall {
		cmd.Options("e")
	}

	return cmd.Run()
}

func (m *installedModel) getPackageInfo() tea.Cmd {
	name, err := m.getSelectedPackageName()
	if err != nil {
		return nil
	}

	return cmd.NewCommand().
		Operation("Q").
		Options("i").
		Arguments(name).
		Target(PackageInfo).
		Run()
}

func (m *installedModel) upgradeAll() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	return cmd.NewCommand().
		Operation("S").
		Options("y", "u").
		Arguments("--noconfirm").
		Target(Background).
		Callback(m.getInstalledPackages).
		Run()
}

func (m *installedModel) upgradeSelected() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	name, err := m.getSelectedPackageName()
	if err != nil {
		return nil
	}

	return cmd.NewCommand().
		Operation("S").
		Options("y", "u").
		Arguments(name, "--noconfirm").
		Target(Background).
		Run()
}

func (m *installedModel) removeSelected() tea.Cmd {
	if m.searchInput.Focused() {
		return nil
	}

	name, err := m.getSelectedPackageName()
	if err != nil {
		return nil
	}

	return cmd.NewCommand().
		Operation("R").
		Options("s").
		Arguments(name, "--noconfirm").
		Target(Background).
		Run()
}

func (m *installedModel) getSelectedPackageName() (string, error) {
	if len(m.visiblePackageLines) == 0 {
		return "", errors.New("No packages in list")
	}

	return strings.TrimSuffix(m.packageLines[m.visiblePackageLines[m.listCursor]], "\n"), nil
}
