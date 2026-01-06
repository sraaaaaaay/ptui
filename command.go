package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

type StreamTarget uint8

type CommandStartMsg struct {
	CommandId     int
	Target        StreamTarget
	Command       *exec.Cmd
	IsLongRunning bool
}

type CommandChunkMsg struct {
	CommandId int
	Target    StreamTarget
	Lines     []string
	IsError   bool
}

type CommandDoneMsg struct {
	CommandId int
	Target    StreamTarget
	Err       error
}

var mutex sync.Mutex

func tryGetDbLock() bool {
	if _, err := os.Stat("/var/lib/pacman/db.lck"); errors.Is(err, os.ErrNotExist) {
		return true
	} else {
		return false
	}
}

func getInstalledPackages() tea.Cmd {
	cmd := []string{"-Qq"}
	return startCommand(cmd, PackageList, true)
}

func getExplicitlyInstalledPackages() tea.Cmd {
	cmd := []string{"-Qqe"}
	return startCommand(cmd, PackageList, true)
}

func getPackageInfo(name string) tea.Cmd {
	cmd := []string{"-Qi", strings.TrimSuffix(name, "\n")}
	return startCommand(cmd, PackageInfo, false)
}

func upgradeAll() tea.Cmd {
	cmd := []string{"-Syu", "--noconfirm"}
	return startCommand(cmd, Background, true)
}

func upgradeSelected(name string) tea.Cmd {
	cmd := []string{"-Syu", name, "--noconfirm"}
	return startCommand(cmd, Background, true)
}

func removeSelected(name string) tea.Cmd {
	cmd := []string{"-Rs", strings.TrimSuffix(name, "\n"), "--noconfirm"}
	return startCommand(cmd, Background, true)
}

func searchPackageDatabase(searchText string) tea.Cmd {
	cmd := []string{"-Ssq", searchText, "--noconfirm"}
	return startCommand(cmd, SearchResultList, true)
}

var nextId atomic.Int32

func startCommand(args []string, target StreamTarget, isLongRunning bool) tea.Cmd {
	id := (int)(nextId.Load())
	nextId.Add(1)

	// The Bubble Tea runtime does not guarantee an execution
	// order when batching tea.Cmds. In order to distinguish
	// concurrent pTUI commands from an external database lock,
	// checks for both are required.
	mutex.Lock()

	// It is possible that the pacman database is locked
	// by an another, such as a cron job or another instance
	// of pTUI. If this is the case, exit early
	if !tryGetDbLock() {
		return func() tea.Msg {
			return CommandDoneMsg{
				CommandId: id,
				Target:    target,
				Err:       errors.New("could not acquire package database lock"),
			}
		}
	}

	return func() tea.Msg {
		cmd := exec.Command("pacman", args...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return CommandDoneMsg{CommandId: id, Target: target, Err: err}
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return CommandDoneMsg{CommandId: id, Target: target, Err: err}
		}

		err = cmd.Start()
		if err != nil {
			return CommandDoneMsg{Err: err}
		}

		// We need to ensure that the pipes aren't closed until
		// we're finished reading from them, rather than
		// simply waiting for the command to complete.
		var pipes sync.WaitGroup
		pipes.Add(2)

		go func() {
			defer pipes.Done()
			streamLines(id, target, stdout, false)
		}()

		go func() {
			defer pipes.Done()
			streamLines(id, target, stderr, true)
		}()

		go func() {
			pipes.Wait()
			program.Send(CommandDoneMsg{CommandId: id, Target: target, Err: cmd.Wait()})
		}()

		mutex.Unlock()
		return CommandStartMsg{CommandId: id, Target: target, Command: cmd, IsLongRunning: isLongRunning}

	}
}

func streamLines(id int, target StreamTarget, reader io.Reader, isStdErr bool) {
	sc := bufio.NewScanner(reader)
	const batchSize = 100

	var batch []string
	for sc.Scan() {
		batch = append(batch, sc.Text()+"\n")

		if len(batch) >= batchSize {
			program.Send(CommandChunkMsg{CommandId: id, Target: target, Lines: batch, IsError: isStdErr})
			batch = make([]string, 0, batchSize)
		}
	}

	if len(batch) > 0 {
		program.Send(CommandChunkMsg{
			CommandId: id,
			Target:    target,
			Lines:     batch,
			IsError:   isStdErr,
		})
	}

	if err := sc.Err(); err != nil {
		program.Send(CommandChunkMsg{
			CommandId: id,
			Target:    target,
			Lines:     []string{fmt.Sprintf("%s", err)},
			IsError:   true,
		})
	}
}
