// Package compose models a multi-agent compose document.
//
// The on-disk format is line-oriented so the standard library is sufficient:
//
//	# AgentCompose
//	COMPOSE planner-team
//	AGENT planner FILE=./agents/planner/Agentfile
//	AGENT coder FILE=./agents/coder/Agentfile DEPENDS_ON=planner
//	AGENT reviewer FILE=./agents/reviewer/Agentfile DEPENDS_ON=coder
//
// Each AGENT entry references an Agentfile by relative path. A topological
// order respecting DEPENDS_ON is exposed via Plan so the runtime can start
// agents deterministically.
package compose

import (
	"container/heap"
	"errors"
	"fmt"
	"strings"
)

// Service is one named agent in the compose document.
type Service struct {
	Name      string   `json:"name"`
	File      string   `json:"file"`
	DependsOn []string `json:"depends_on,omitempty"`
}

// Document is the parsed AgentCompose definition.
type Document struct {
	Name     string    `json:"name"`
	Services []Service `json:"services"`
}

// ErrCyclicDependency is returned by Plan when DEPENDS_ON forms a cycle.
var ErrCyclicDependency = errors.New("cyclic depends_on graph")

// Service returns the named service or false when missing. O(N) — Plan
// builds an O(1) lookup internally and does not call this in its loop.
func (d Document) Service(name string) (Service, bool) {
	for _, service := range d.Services {
		if service.Name == name {
			return service, true
		}
	}
	return Service{}, false
}

// Validate ensures every DEPENDS_ON reference exists and that service names are
// unique. Cycle detection happens lazily during Plan to keep error messages
// targeted at the actual order computation.
func (d Document) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("COMPOSE name is required")
	}
	if len(d.Services) == 0 {
		return errors.New("at least one AGENT is required")
	}

	seen := map[string]struct{}{}
	for _, service := range d.Services {
		if strings.TrimSpace(service.Name) == "" {
			return errors.New("AGENT name is required")
		}
		if _, exists := seen[service.Name]; exists {
			return fmt.Errorf("duplicate AGENT %q", service.Name)
		}
		seen[service.Name] = struct{}{}
		if strings.TrimSpace(service.File) == "" {
			return fmt.Errorf("AGENT %q FILE= is required", service.Name)
		}
	}
	for _, service := range d.Services {
		for _, dep := range service.DependsOn {
			if _, ok := seen[dep]; !ok {
				return fmt.Errorf("AGENT %q DEPENDS_ON unknown service %q", service.Name, dep)
			}
			if dep == service.Name {
				return fmt.Errorf("AGENT %q depends on itself", service.Name)
			}
		}
	}
	return nil
}

// Plan returns services in dependency order using Kahn's algorithm with a
// min-heap by name. Total cost is O((V + E) log V) — every node enters and
// leaves the heap once and each edge contributes one indegree decrement.
// Equal-rank nodes are emitted alphabetically so the output is
// deterministic for tests and trace logs.
func (d Document) Plan() ([]Service, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}

	byName := make(map[string]Service, len(d.Services))
	indegree := make(map[string]int, len(d.Services))
	graph := make(map[string][]string, len(d.Services))
	for _, service := range d.Services {
		byName[service.Name] = service
		indegree[service.Name] = 0
	}
	for _, service := range d.Services {
		for _, dep := range service.DependsOn {
			indegree[service.Name]++
			graph[dep] = append(graph[dep], service.Name)
		}
	}

	ready := &nameHeap{}
	heap.Init(ready)
	for name, count := range indegree {
		if count == 0 {
			heap.Push(ready, name)
		}
	}

	ordered := make([]Service, 0, len(d.Services))
	for ready.Len() > 0 {
		current := heap.Pop(ready).(string)
		ordered = append(ordered, byName[current])
		for _, next := range graph[current] {
			indegree[next]--
			if indegree[next] == 0 {
				heap.Push(ready, next)
			}
		}
	}
	if len(ordered) != len(d.Services) {
		return nil, ErrCyclicDependency
	}
	return ordered, nil
}

// nameHeap is a string min-heap satisfying container/heap.Interface. It
// orders names lexicographically so Plan produces deterministic output.
type nameHeap []string

func (h nameHeap) Len() int           { return len(h) }
func (h nameHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h nameHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *nameHeap) Push(value any)    { *h = append(*h, value.(string)) }
func (h *nameHeap) Pop() any {
	old := *h
	n := len(old)
	value := old[n-1]
	*h = old[:n-1]
	return value
}
