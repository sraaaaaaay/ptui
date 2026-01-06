package main

import tea "github.com/charmbracelet/bubbletea"

type handler[T tea.Model, M tea.Msg] func(T, M) tea.Cmd

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
