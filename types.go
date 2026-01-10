package main

import (
	"ptui/command"

	tea "github.com/charmbracelet/bubbletea"
)

type MessageRouter[T tea.Model, M tea.Msg] map[command.StreamTarget]func(T, M) tea.Cmd

type ContentRectMsg struct {
	Width, Height int
}

type hotkeyBinding struct {
	shortcut    string
	description string
	command     func() tea.Cmd
}

type HotkeyPressedMsg struct {
	hotkey hotkeyBinding
}
