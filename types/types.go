package types

import (
	tea "github.com/charmbracelet/bubbletea"
)

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
