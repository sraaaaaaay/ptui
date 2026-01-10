package main

import (
	"fmt"
	"os"
	"ptui/command"

	tea "github.com/charmbracelet/bubbletea"
)

const ROOT_USER_ID = 0
const APP_NAME = "pTUI"

var Program *tea.Program

func main() {
	if os.Geteuid() != ROOT_USER_ID {
		fmt.Printf("%s requires root privileges. Please run as sudo.\n", APP_NAME)
		os.Exit(1)
	}

	Program = tea.NewProgram(initialModel())
	command.Program = Program

	if _, err := Program.Run(); err != nil {
		fmt.Printf("Error with the program, exiting: %v\n", err)
		os.Exit(1)
	}
}
