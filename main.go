package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const ROOT_USER_ID = 0
const APP_NAME = "pTUI"

var program *tea.Program

func main() {
	if os.Geteuid() != ROOT_USER_ID {
		fmt.Printf("%s requires root privileges. Please run as sudo.\n", APP_NAME)
		os.Exit(1)
	}

	program = tea.NewProgram(initialModel())
	if _, err := program.Run(); err != nil {
		fmt.Printf("Error with the program, exiting: %v\n", err)
		os.Exit(1)
	}
}
