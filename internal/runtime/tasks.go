// Package runtime is the in-process implementation of the Agentfile runtime
// contract documented in docs/agentfile.md. agentd embeds this package to
// expose /health, /status, /tasks, and /tasks/{id} backed by an in-memory
// task store and a worker goroutine that drives the configured model client.
package runtime

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Status is the lifecycle state of a single task.
type Status string

const (
	StatusQueued  Status = "queued"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusError   Status = "error"
)

// Default queue and retention bounds. A long-running agentd hosting a chatty
// caller would otherwise grow its in-memory task map without bound; the
// retention cap evicts oldest *terminal* tasks (done / error) so live work
// is never silently dropped.
const (
	DefaultQueueCapacity   = 64
	DefaultRetainTerminals = 256
)

// Task is one unit of work submitted to the agent runtime.
type Task struct {
	ID          string    `json:"id"`
	Status      Status    `json:"status"`
	Prompt      string    `json:"prompt"`
	System      string    `json:"system,omitempty"`
	Result      string    `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// terminal returns true when the task is no longer active.
func (t *Task) terminal() bool {
	return t.Status == StatusDone || t.Status == StatusError
}

// Store is a thread-safe collection of Tasks. The runtime worker reads queued
// tasks via Next, transitions them through Mark calls, and the HTTP server
// reads via Get / List.
type Store struct {
	mu              sync.Mutex
	tasks           map[string]*Task
	order           []string
	queue           chan string
	now             func() time.Time
	idSeed          int64
	retainTerminals int
	closeOnce       sync.Once
}

// ErrNotFound is returned by Store.Get when the id is unknown.
var ErrNotFound = errors.New("task not found")

// NewStore returns an empty Store. queueCapacity bounds how many queued tasks
// can wait before Submit returns "queue is full"; retainTerminals bounds how
// many done / error tasks the store retains. Zero or negative values fall
// back to the package defaults.
func NewStore(queueCapacity int, retainTerminals int) *Store {
	if queueCapacity <= 0 {
		queueCapacity = DefaultQueueCapacity
	}
	if retainTerminals <= 0 {
		retainTerminals = DefaultRetainTerminals
	}
	return &Store{
		tasks:           map[string]*Task{},
		queue:           make(chan string, queueCapacity),
		now:             time.Now,
		retainTerminals: retainTerminals,
	}
}

// Submit creates a new queued Task. The returned Task is a snapshot.
func (s *Store) Submit(prompt string, system string) (Task, error) {
	if prompt == "" {
		return Task{}, errors.New("prompt is required")
	}
	s.mu.Lock()
	s.idSeed++
	id := fmt.Sprintf("task-%d", s.idSeed)
	now := s.now().UTC()
	task := &Task{
		ID:        id,
		Status:    StatusQueued,
		Prompt:    prompt,
		System:    system,
		CreatedAt: now,
	}
	s.tasks[id] = task
	s.order = append(s.order, id)
	s.mu.Unlock()

	select {
	case s.queue <- id:
		return *task, nil
	default:
		s.mu.Lock()
		task.Status = StatusError
		task.Error = "queue is full"
		task.CompletedAt = s.now().UTC()
		snapshot := *task
		s.evictTerminalsLocked()
		s.mu.Unlock()
		return snapshot, errors.New("queue is full")
	}
}

// Next blocks until a queued task is available or the queue is closed.
// The returned bool is false when the queue has been closed and drained.
func (s *Store) Next() (Task, bool) {
	id, ok := <-s.queue
	if !ok {
		return Task{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	task, exists := s.tasks[id]
	if !exists {
		return Task{}, false
	}
	return *task, true
}

// MarkRunning transitions a queued task to running.
func (s *Store) MarkRunning(id string) error {
	return s.update(id, func(task *Task) {
		task.Status = StatusRunning
		task.StartedAt = s.now().UTC()
	})
}

// MarkDone records a successful completion with the model's result.
func (s *Store) MarkDone(id string, result string) error {
	return s.update(id, func(task *Task) {
		task.Status = StatusDone
		task.Result = result
		task.CompletedAt = s.now().UTC()
	})
}

// MarkError records a failure.
func (s *Store) MarkError(id string, err error) error {
	return s.update(id, func(task *Task) {
		task.Status = StatusError
		if err != nil {
			task.Error = err.Error()
		}
		task.CompletedAt = s.now().UTC()
	})
}

// Get returns a snapshot of one task.
func (s *Store) Get(id string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return *task, nil
}

// List returns every task in submission order.
func (s *Store) List() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks := make([]Task, 0, len(s.order))
	for _, id := range s.order {
		if task, ok := s.tasks[id]; ok {
			tasks = append(tasks, *task)
		}
	}
	return tasks
}

// Counts groups tasks by status. Used by /status.
func (s *Store) Counts() map[Status]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[Status]int{
		StatusQueued:  0,
		StatusRunning: 0,
		StatusDone:    0,
		StatusError:   0,
	}
	for _, task := range s.tasks {
		counts[task.Status]++
	}
	return counts
}

// Close shuts down the queue. Safe to call multiple times.
func (s *Store) Close() {
	s.closeOnce.Do(func() {
		close(s.queue)
	})
}

func (s *Store) update(id string, mutate func(*Task)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	if !ok {
		return ErrNotFound
	}
	mutate(task)
	if task.terminal() {
		s.evictTerminalsLocked()
	}
	return nil
}

// evictTerminalsLocked drops the oldest terminal tasks from order + tasks
// when the count of retained terminals exceeds retainTerminals. Active
// (queued / running) tasks are never evicted. Caller must hold s.mu.
func (s *Store) evictTerminalsLocked() {
	if s.retainTerminals <= 0 {
		return
	}
	terminalCount := 0
	for _, id := range s.order {
		if task, ok := s.tasks[id]; ok && task.terminal() {
			terminalCount++
		}
	}
	if terminalCount <= s.retainTerminals {
		return
	}
	overflow := terminalCount - s.retainTerminals

	keep := make([]string, 0, len(s.order))
	for _, id := range s.order {
		task, ok := s.tasks[id]
		if !ok {
			continue
		}
		if overflow > 0 && task.terminal() {
			delete(s.tasks, id)
			overflow--
			continue
		}
		keep = append(keep, id)
	}
	s.order = keep
}
