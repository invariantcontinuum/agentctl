package health

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type stubClient struct {
	responses map[string]*http.Response
	errors    map[string]error
	requests  []string
}

func (c *stubClient) Do(request *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, request.URL.Path)
	if err, ok := c.errors[request.URL.Path]; ok {
		return nil, err
	}
	if response, ok := c.responses[request.URL.Path]; ok {
		return response, nil
	}
	return nil, errors.New("unexpected path: " + request.URL.Path)
}

func newResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
}

func TestRunProbesEveryEndpoint(t *testing.T) {
	client := &stubClient{
		responses: map[string]*http.Response{
			"/health": newResponse(200, `{"ok":true}`),
			"/status": newResponse(200, `{"loop":"react"}`),
			"/tasks":  newResponse(200, `[]`),
		},
	}
	probe := NewProbeWithClient(client, 0)
	report, err := probe.Run(context.Background(), "http://localhost:8088")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(report.Probes) != len(DefaultEndpoints) {
		t.Fatalf("len(Probes) = %d, want %d", len(report.Probes), len(DefaultEndpoints))
	}
	for index, endpoint := range DefaultEndpoints {
		if report.Probes[index].Endpoint != endpoint {
			t.Fatalf("probe[%d].Endpoint = %q, want %q", index, report.Probes[index].Endpoint, endpoint)
		}
		if !report.Probes[index].OK {
			t.Fatalf("probe[%d].OK = false, want true", index)
		}
	}
}

func TestRunCapturesErrorPerEndpoint(t *testing.T) {
	client := &stubClient{
		responses: map[string]*http.Response{
			"/health": newResponse(200, "ok"),
			"/status": newResponse(500, "internal"),
		},
		errors: map[string]error{
			"/tasks": errors.New("connection refused"),
		},
	}
	probe := NewProbeWithClient(client, 0)
	report, err := probe.Run(context.Background(), "http://localhost:8088")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !report.Probes[0].OK || report.Probes[1].OK {
		t.Fatalf("expected first OK and second failing: %+v", report.Probes)
	}
	if report.Probes[2].Error == "" {
		t.Fatalf("expected error on third probe, got %+v", report.Probes[2])
	}
}

func TestRunRejectsEmptyBaseURL(t *testing.T) {
	probe := NewProbeWithClient(&stubClient{}, 0)
	if _, err := probe.Run(context.Background(), ""); err == nil {
		t.Fatal("Run returned nil error for empty baseURL")
	}
}

func TestWriteReport(t *testing.T) {
	report := Report{
		BaseURL: "http://localhost:8088",
		Probes: []Result{
			{Endpoint: "/health", URL: "http://localhost:8088/health", StatusCode: 200, OK: true},
			{Endpoint: "/status", URL: "http://localhost:8088/status", Error: "boom"},
		},
	}
	var buffer bytes.Buffer
	if err := WriteReport(&buffer, report); err != nil {
		t.Fatalf("WriteReport returned error: %v", err)
	}
	output := buffer.String()
	if !strings.Contains(output, "BASE http://localhost:8088") {
		t.Fatalf("missing base line: %s", output)
	}
	if !strings.Contains(output, " OK  /health") {
		t.Fatalf("missing health line: %s", output)
	}
	if !strings.Contains(output, "FAIL /status") {
		t.Fatalf("missing status fail line: %s", output)
	}
	if !strings.Contains(output, "error: boom") {
		t.Fatalf("missing error detail: %s", output)
	}
}
