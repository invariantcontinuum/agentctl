package driver

import (
	"context"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

type Process struct {
	PID int
}

type StartOptions struct {
	LogPath string
	WorkDir string
}

type Driver interface {
	Start(context.Context, agent.Config, StartOptions) (Process, error)
	Stop(context.Context, Process) error
	Status(context.Context, Process) (Status, error)
}
