package types

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type StatusProvider interface {
	StatusBar() string
}

type ChildModel interface {
	tea.Model
	StatusProvider
}

type MessageRouter[T tea.Model, M tea.Msg] map[StreamTarget]func(T, M) tea.Cmd
type StreamTarget uint8

type ContentRectMsg struct {
	Width, Height int
}

type HotkeyBinding struct {
	Shortcut    string
	Description string
	Command     func() tea.Cmd
}

type HotkeyPressedMsg struct {
	Hotkey HotkeyBinding
}

type PackageListModel interface {
	SearchInput() *textinput.Model
	Hotkeys() map[string]HotkeyBinding
	AddCommand(tea.Cmd)
	ResetCursor()
}
