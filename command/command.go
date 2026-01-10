package command

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

var Program *tea.Program

const (
	PackageList StreamTarget = iota
	PackageInfo
	Background
	SearchResultList
)

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

type Command struct {
	operation string
	options   []string
	args      []string
	target    StreamTarget
}

var mutex sync.Mutex

func tryGetDbLock() bool {
	if _, err := os.Stat("/var/lib/pacman/db.lck"); errors.Is(err, os.ErrNotExist) {
		return true
	} else {
		return false
	}
}

func NewCommand() *Command {
	return &Command{}
}

func (c *Command) WithOperation(op string) *Command {
	c.operation = "-" + op
	return c
}

func (c *Command) WithOptions(opts ...string) *Command {
	c.options = append(c.options, opts...)
	return c
}

func (c *Command) WithArguments(args ...string) *Command {
	c.args = append(c.args, args...)
	return c
}

func (c *Command) WithTarget(target StreamTarget) *Command {
	c.target = target
	return c
}

func (c *Command) Run() tea.Cmd {
	var allArgs []string
	mainOp := c.operation + strings.Join(c.options, "")
	allArgs = append(allArgs, mainOp)
	allArgs = append(allArgs, c.args...)

	return startCommand(allArgs, c.target, true)
}

func GetExplicitlyInstalledPackages() tea.Cmd {
	cmd := []string{"Qqe", "--noconfirm"}
	return startCommand(cmd, PackageList, true)
}

func GetPackageInfo(name string) tea.Cmd {
	cmd := []string{"-Qi", name, "--noconfirm"}
	return startCommand(cmd, PackageInfo, false)
}

func UpgradeAll() tea.Cmd {
	cmd := []string{"-Syu", "--noconfirm"}
	return startCommand(cmd, Background, true)
}

func UpgradeSelected(name string) tea.Cmd {
	cmd := []string{"-Syu", name, "--noconfirm"}
	return startCommand(cmd, Background, true)
}

func RemoveSelected(name string) tea.Cmd {
	cmd := []string{"-Rs", strings.TrimSuffix(name, "\n"), "--noconfirm"}
	return startCommand(cmd, Background, true)
}

var nextId atomic.Int32

func startCommand(args []string, target StreamTarget, isLongRunning bool) tea.Cmd {
	id := (int)(nextId.Load())
	nextId.Add(1)

	// The Bubble Tea runtime does not guarantee an execution
	// order when batching tea.Cmds. In order to distinguish
	// concurrent pTUI commands from an external database lock,
	// checks for both are required.
	return func() tea.Msg {
		mutex.Lock()
		defer mutex.Unlock()

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
			Program.Send(CommandDoneMsg{CommandId: id, Target: target, Err: cmd.Wait()})
		}()

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
			Program.Send(CommandChunkMsg{CommandId: id, Target: target, Lines: batch, IsError: isStdErr})
			batch = make([]string, 0, batchSize)
		}
	}

	if len(batch) > 0 {
		Program.Send(CommandChunkMsg{
			CommandId: id,
			Target:    target,
			Lines:     batch,
			IsError:   isStdErr,
		})
	}

	if err := sc.Err(); err != nil {
		Program.Send(CommandChunkMsg{
			CommandId: id,
			Target:    target,
			Lines:     []string{fmt.Sprintf("%s", err)},
			IsError:   true,
		})
	}
}
