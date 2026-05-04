package driver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

type Local struct{}

func NewLocal() *Local {
	return &Local{}
}

func (Local) Start(_ context.Context, config agent.Config, options StartOptions) (Process, error) {
	program, args := config.Command()
	if program == "" {
		return Process{}, fmt.Errorf("agent command is empty")
	}
	if options.WorkDir == "" {
		options.WorkDir = "."
	}
	if err := os.MkdirAll(filepath.Dir(options.LogPath), 0o755); err != nil {
		return Process{}, err
	}

	logFile, err := os.OpenFile(options.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return Process{}, err
	}
	defer logFile.Close()

	command := exec.Command(program, args...)
	command.Dir = options.WorkDir
	command.Env = config.EnvList(os.Environ())
	command.Stdout = logFile
	command.Stderr = logFile

	if err := command.Start(); err != nil {
		return Process{}, err
	}
	return Process{PID: command.Process.Pid}, nil
}

func (Local) Stop(_ context.Context, process Process) error {
	if process.PID <= 0 {
		return fmt.Errorf("pid must be positive")
	}
	return syscall.Kill(process.PID, syscall.SIGTERM)
}

func (Local) Status(_ context.Context, process Process) (Status, error) {
	if process.PID <= 0 {
		return StatusStopped, nil
	}
	if err := syscall.Kill(process.PID, 0); err != nil {
		return StatusStopped, nil
	}
	return StatusRunning, nil
}
