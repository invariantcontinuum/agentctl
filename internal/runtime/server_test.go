package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/agentsdk"
	"github.com/invariantcontinuum/agentctl/internal/logging"
)

// stubModel is a deterministic agentsdk.ModelClient for runtime tests. It
// records every call and emits one assistant text block built from the
// most recent user message, terminating the loop after the first response.
type stubModel struct {
	provider string
	reply    string
	err      error
	calls    int
}

func (s *stubModel) Provider() string { return s.provider }

func (s *stubModel) Generate(_ context.Context, request agentsdk.GenerateRequest) (agentsdk.GenerateResponse, error) {
	s.calls++
	if s.err != nil {
		return agentsdk.GenerateResponse{}, s.err
	}
	prompt := ""
	for index := len(request.Messages) - 1; index >= 0; index-- {
		message := request.Messages[index]
		if message.Role == agentsdk.RoleUser {
			prompt = message.FirstText()
			if prompt != "" {
				break
			}
		}
	}
	text := s.reply
	if prompt != "" {
		text = s.reply + ":" + prompt
	}
	return agentsdk.GenerateResponse{
		Provider:   s.provider,
		Content:    []agentsdk.ContentBlock{agentsdk.TextBlock(text)},
		StopReason: agentsdk.StopReasonEndTurn,
	}, nil
}

func newTestRuntime(model agentsdk.ModelClient) *Runtime {
	config := agent.Config{Name: "tester", Type: "planner", Loop: agent.Loop{Name: "react", MaxSteps: 4}}
	logger := logging.New(&bytes.Buffer{}, logging.LevelError)
	rt := New(config, Options{Address: "127.0.0.1:0", Model: model, Logger: logger, Capacity: 4})
	return rt
}

func TestServerHealthOK(t *testing.T) {
	rt := newTestRuntime(&stubModel{provider: "stub"})

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	rt.handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestServerSubmitsTaskAndReadsBack(t *testing.T) {
	rt := newTestRuntime(&stubModel{provider: "stub", reply: "answer"})

	body := strings.NewReader(`{"prompt":"hello","system":"sys"}`)
	postReq := httptest.NewRequest(http.MethodPost, "/tasks", body)
	postRec := httptest.NewRecorder()
	rt.handler().ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusAccepted {
		t.Fatalf("post status = %d, want 202: %s", postRec.Code, postRec.Body.String())
	}

	var posted Task
	if err := json.Unmarshal(postRec.Body.Bytes(), &posted); err != nil {
		t.Fatalf("decode posted: %v", err)
	}
	if posted.Status != StatusQueued || posted.ID == "" {
		t.Fatalf("unexpected task after submit: %+v", posted)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/tasks/"+posted.ID, nil)
	getRec := httptest.NewRecorder()
	rt.handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getRec.Code)
	}
}

func TestServerListsTasks(t *testing.T) {
	rt := newTestRuntime(&stubModel{provider: "stub"})
	if _, err := rt.store.Submit("first", ""); err != nil {
		t.Fatalf("submit: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	recorder := httptest.NewRecorder()
	rt.handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body struct {
		Tasks []Task `json:"tasks"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Tasks) != 1 || body.Tasks[0].Prompt != "first" {
		t.Fatalf("unexpected list: %+v", body)
	}
}

func TestServerMissingTaskIs404(t *testing.T) {
	rt := newTestRuntime(&stubModel{provider: "stub"})
	request := httptest.NewRequest(http.MethodGet, "/tasks/does-not-exist", nil)
	recorder := httptest.NewRecorder()
	rt.handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestServerRejectsBadMethods(t *testing.T) {
	rt := newTestRuntime(&stubModel{provider: "stub"})
	request := httptest.NewRequest(http.MethodPost, "/health", nil)
	recorder := httptest.NewRecorder()
	rt.handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}

func TestRuntimeWorkerProcessesQueue(t *testing.T) {
	model := &stubModel{provider: "stub", reply: "ok"}
	rt := newTestRuntime(model)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go rt.runWorker(ctx)

	if _, err := rt.store.Submit("ping", ""); err != nil {
		t.Fatalf("submit: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tasks := rt.store.List()
		if len(tasks) == 1 && tasks[0].Status == StatusDone {
			if tasks[0].Result != "ok:ping" {
				t.Fatalf("result = %q, want %q", tasks[0].Result, "ok:ping")
			}
			rt.store.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("worker did not finish task in time; calls=%d", model.calls)
}

func TestRuntimeWorkerRecordsModelError(t *testing.T) {
	model := &stubModel{provider: "stub", err: errors.New("boom")}
	rt := newTestRuntime(model)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go rt.runWorker(ctx)

	task, err := rt.store.Submit("ping", "")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := rt.store.Get(task.ID)
		if got.Status == StatusError {
			if got.Error == "" {
				t.Fatalf("expected error message, got %+v", got)
			}
			rt.store.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("worker never recorded error")
}
