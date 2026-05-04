package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/invariantcontinuum/agentctl/internal/agent"
	"github.com/invariantcontinuum/agentctl/internal/health"
	"github.com/invariantcontinuum/agentctl/internal/trace"
)

// health probes the runtime contract endpoints for an agent. The endpoint
// resolution order is: command-line --url override, Agentfile ENDPOINT named
// "http", then the first ENDPOINT entry.
func (a *App) health(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("health", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	urlOverride := flags.String("url", "", "override base URL (default: ENDPOINT http or first endpoint)")
	jsonOutput := flags.Bool("json", false, "emit JSON report")
	if err := flags.Parse(args); err != nil {
		return err
	}
	id, err := requiredID("health", flags.Args())
	if err != nil {
		return err
	}
	instance, err := a.repo.Find(id)
	if err != nil {
		return err
	}

	baseURL := *urlOverride
	if baseURL == "" {
		resolved, err := resolveBaseURL(instance.Config.Endpoints)
		if err != nil {
			return err
		}
		baseURL = resolved
	}

	probe := a.healthProbeFor(id)
	report, err := probe.Run(ctx, baseURL)
	if err != nil {
		return err
	}

	if err := a.writeTrace(instance.TracePath, trace.Event{
		Time:   a.now().UTC(),
		Kind:   trace.KindHealth,
		Agent:  id,
		Detail: fmt.Sprintf("base=%s", baseURL),
		Fields: healthSummaryFields(report),
	}); err != nil {
		return err
	}

	if *jsonOutput {
		return health.WriteReportJSON(a.out, report)
	}
	return health.WriteReport(a.out, report)
}

func resolveBaseURL(endpoints []agent.Endpoint) (string, error) {
	if len(endpoints) == 0 {
		return "", fmt.Errorf("agent has no ENDPOINT entries; pass --url")
	}
	for _, endpoint := range endpoints {
		if endpoint.Name == "http" {
			return agent.EndpointURL(endpoint), nil
		}
	}
	return agent.EndpointURL(endpoints[0]), nil
}

func healthSummaryFields(report health.Report) map[string]string {
	ok, fail := 0, 0
	for _, probe := range report.Probes {
		if probe.OK {
			ok++
		} else {
			fail++
		}
	}
	return map[string]string{"ok": fmt.Sprintf("%d", ok), "fail": fmt.Sprintf("%d", fail)}
}
