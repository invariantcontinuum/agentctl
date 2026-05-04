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
	"errors"
	"fmt"
	"sort"
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

// Service returns the named service or false when missing.
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

// Plan returns service names in dependency order (Kahn's algorithm). Equal-rank
// nodes are sorted alphabetically so the output is deterministic for tests.
func (d Document) Plan() ([]Service, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}

	indegree := map[string]int{}
	graph := map[string][]string{}
	for _, service := range d.Services {
		indegree[service.Name] = 0
	}
	for _, service := range d.Services {
		for _, dep := range service.DependsOn {
			indegree[service.Name]++
			graph[dep] = append(graph[dep], service.Name)
		}
	}

	ready := make([]string, 0)
	for name, count := range indegree {
		if count == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)

	ordered := make([]Service, 0, len(d.Services))
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]

		service, _ := d.Service(current)
		ordered = append(ordered, service)

		nexts := append([]string{}, graph[current]...)
		sort.Strings(nexts)
		for _, next := range nexts {
			indegree[next]--
			if indegree[next] == 0 {
				ready = append(ready, next)
				sort.Strings(ready)
			}
		}
	}
	if len(ordered) != len(d.Services) {
		return nil, ErrCyclicDependency
	}
	return ordered, nil
}
