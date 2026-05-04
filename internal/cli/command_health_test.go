package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/health"
	"github.com/invariantcontinuum/agentctl/internal/store"
)

type stubHealthHTTP struct {
	statuses map[string]int
}

func (s stubHealthHTTP) Do(request *http.Request) (*http.Response, error) {
	status, ok := s.statuses[request.URL.Path]
	if !ok {
		status = 404
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Header:     http.Header{},
	}, nil
}

func TestHealthProbesEndpointAndTraces(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	tracePath := filepath.Join(dir, "planner.trace")
	if err := repo.Save(store.Instance{
		ID:        "planner-1",
		Type:      "planner",
		TracePath: tracePath,
		Config: agent.Config{
			Name:      "planner",
			Type:      "planner",
			Endpoints: []agent.Endpoint{{Name: "http", Scheme: "http", Host: "localhost", Port: 8088}},
			Loop:      agent.Loop{Name: "react", MaxSteps: 1},
			Exec:      []string{"sleep", "1"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{})
	app.now = func() time.Time { return now }
	app.healthProbeFor = func(string) *health.Probe {
		return health.NewProbeWithClient(stubHealthHTTP{statuses: map[string]int{"/health": 200, "/status": 200, "/tasks": 503}}, 0)
	}

	exitCode := app.Run(context.Background(), []string{"health", "planner-1"})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %s", exitCode, errOut.String())
	}
	output := out.String()
	if !strings.Contains(output, " OK  /health") {
		t.Fatalf("output missing health line: %s", output)
	}
	if !strings.Contains(output, "FAIL /tasks") {
		t.Fatalf("output missing tasks fail: %s", output)
	}

	traceContents, err := readAllFromPath(tracePath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(traceContents), `"kind":"health"`) {
		t.Fatalf("trace missing health event: %s", string(traceContents))
	}
}

func TestHealthRequiresEndpointOrURL(t *testing.T) {
	dir := t.TempDir()
	repo := store.NewJSONRepository(filepath.Join(dir, "state.json"))
	now := time.Unix(100, 0).UTC()
	if err := repo.Save(store.Instance{
		ID:        "planner-1",
		Type:      "planner",
		Config:    agent.Config{Name: "planner", Type: "planner", Loop: agent.Loop{Name: "react", MaxSteps: 1}, Exec: []string{"sleep", "1"}},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(&out, &errOut, repo, fakeDriver{})

	exitCode := app.Run(context.Background(), []string{"health", "planner-1"})
	if exitCode == 0 {
		t.Fatal("exitCode = 0, want failure when no ENDPOINT and no --url")
	}
	if !strings.Contains(errOut.String(), "ENDPOINT") {
		t.Fatalf("stderr missing ENDPOINT message: %s", errOut.String())
	}
}
