package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

type Instance struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Status    string       `json:"status"`
	PID       int          `json:"pid,omitempty"`
	Config    agent.Config `json:"config"`
	LogPath   string       `json:"log_path,omitempty"`
	TracePath string       `json:"trace_path,omitempty"`
	WorkDir   string       `json:"work_dir"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

type Repository interface {
	List() ([]Instance, error)
	Find(id string) (Instance, error)
	Save(instance Instance) error
}

type JSONRepository struct {
	path string
}

type fileState struct {
	Instances []Instance `json:"instances"`
}

var ErrNotFound = errors.New("instance not found")

func NewJSONRepository(path string) *JSONRepository {
	return &JSONRepository{path: path}
}

func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "agentctl", "state.json"), nil
}

func (r *JSONRepository) List() ([]Instance, error) {
	state, err := r.load()
	if err != nil {
		return nil, err
	}
	sort.Slice(state.Instances, func(left, right int) bool {
		return state.Instances[left].ID < state.Instances[right].ID
	})
	return state.Instances, nil
}

func (r *JSONRepository) Find(id string) (Instance, error) {
	state, err := r.load()
	if err != nil {
		return Instance{}, err
	}
	for _, instance := range state.Instances {
		if instance.ID == id {
			return instance, nil
		}
	}
	return Instance{}, fmt.Errorf("%w: %s", ErrNotFound, id)
}

func (r *JSONRepository) Save(instance Instance) error {
	state, err := r.load()
	if err != nil {
		return err
	}

	replaced := false
	for index := range state.Instances {
		if state.Instances[index].ID == instance.ID {
			state.Instances[index] = instance
			replaced = true
			break
		}
	}
	if !replaced {
		state.Instances = append(state.Instances, instance)
	}

	return r.save(state)
}

func (r *JSONRepository) load() (fileState, error) {
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{}, nil
	}
	if err != nil {
		return fileState{}, err
	}
	if len(data) == 0 {
		return fileState{}, nil
	}

	var state fileState
	if err := json.Unmarshal(data, &state); err != nil {
		return fileState{}, err
	}
	return state, nil
}

func (r *JSONRepository) save(state fileState) error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tempPath := r.path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, r.path)
}
