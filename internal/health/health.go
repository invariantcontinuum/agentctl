// Package health probes the runtime contract documented in docs/agentfile.md.
//
// The agent runtime is expected to expose:
//
//	GET /health   liveness probe
//	GET /status   reasoning loop status
//	GET /tasks    in-flight + recent tasks
//
// Probe wraps these calls behind a small interface so the CLI can render a
// stable report and tests can replace the transport with a fake.
package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Result is one probe outcome.
type Result struct {
	Endpoint   string `json:"endpoint"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	OK         bool   `json:"ok"`
	Body       string `json:"body,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Report bundles the probe results in stable order.
type Report struct {
	BaseURL string   `json:"base_url"`
	Probes  []Result `json:"probes"`
}

// HTTPClient is the minimal surface Probe needs from http.Client. Tests inject
// a fake.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Probe exercises the runtime contract against baseURL.
type Probe struct {
	client    HTTPClient
	bodyLimit int64
}

// DefaultEndpoints documents the contract paths in deterministic order.
var DefaultEndpoints = []string{"/health", "/status", "/tasks"}

// NewProbe returns a probe with a sensible HTTP timeout. bodyLimit caps how
// much of each response body is recorded so a verbose endpoint cannot blow up
// the report.
func NewProbe(timeout time.Duration, bodyLimit int64) *Probe {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if bodyLimit <= 0 {
		bodyLimit = 4 * 1024
	}
	return &Probe{
		client:    &http.Client{Timeout: timeout},
		bodyLimit: bodyLimit,
	}
}

// NewProbeWithClient is the dependency-injected variant for tests.
func NewProbeWithClient(client HTTPClient, bodyLimit int64) *Probe {
	if bodyLimit <= 0 {
		bodyLimit = 4 * 1024
	}
	return &Probe{client: client, bodyLimit: bodyLimit}
}

// Run probes every endpoint relative to baseURL.
func (p *Probe) Run(ctx context.Context, baseURL string) (Report, error) {
	if strings.TrimSpace(baseURL) == "" {
		return Report{}, errors.New("baseURL is required")
	}
	report := Report{BaseURL: baseURL, Probes: make([]Result, 0, len(DefaultEndpoints))}

	for _, endpoint := range DefaultEndpoints {
		report.Probes = append(report.Probes, p.probe(ctx, baseURL, endpoint))
	}
	return report, nil
}

func (p *Probe) probe(ctx context.Context, baseURL string, endpoint string) Result {
	url := strings.TrimRight(baseURL, "/") + endpoint
	result := Result{Endpoint: endpoint, URL: url}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	response, err := p.client.Do(request)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer response.Body.Close()

	result.StatusCode = response.StatusCode
	result.OK = response.StatusCode >= 200 && response.StatusCode < 300

	body, err := io.ReadAll(io.LimitReader(response.Body, p.bodyLimit))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Body = strings.TrimRight(string(body), "\n")
	return result
}

// WriteReport renders a Report to writer in a stable human-friendly form.
func WriteReport(writer io.Writer, report Report) error {
	if _, err := fmt.Fprintf(writer, "BASE %s\n", report.BaseURL); err != nil {
		return err
	}
	for _, probe := range report.Probes {
		marker := "FAIL"
		if probe.OK {
			marker = " OK "
		}
		if _, err := fmt.Fprintf(writer, "%s %-7s %d %s\n", marker, probe.Endpoint, probe.StatusCode, probe.URL); err != nil {
			return err
		}
		if probe.Error != "" {
			if _, err := fmt.Fprintf(writer, "    error: %s\n", probe.Error); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteReportJSON renders Report as indented JSON.
func WriteReportJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
