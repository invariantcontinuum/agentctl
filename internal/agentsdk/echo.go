package agentsdk

import (
	"context"
	"strings"
)

// EchoClient is a deterministic ModelClient used as a fallback when no
// provider is configured. It mirrors the most recent user input back as
// assistant text and never emits tool_use, so tests that don't care about
// model behaviour can still drive the Agent loop end to end.
type EchoClient struct {
	provider string
}

// NewEchoClient returns an EchoClient that records the provider name it
// stands in for so /status surfaces "echo:anthropic" rather than just
// "echo".
func NewEchoClient(provider string) *EchoClient {
	if provider == "" {
		provider = "echo"
	}
	return &EchoClient{provider: provider}
}

// Provider implements ModelClient.
func (c *EchoClient) Provider() string { return "echo:" + c.provider }

// Generate implements ModelClient. It concatenates the latest user message's
// text, prefixes it with "[echo]" (and the system prompt when present),
// and returns end_turn so the Agent loop terminates immediately.
func (c *EchoClient) Generate(_ context.Context, request GenerateRequest) (GenerateResponse, error) {
	latest := ""
	for index := len(request.Messages) - 1; index >= 0; index-- {
		message := request.Messages[index]
		if message.Role == RoleUser {
			text := message.FirstText()
			if text != "" {
				latest = text
				break
			}
		}
	}
	parts := []string{"[echo]"}
	if request.System != "" {
		parts = append(parts, "system="+strings.SplitN(request.System, "\n", 2)[0])
	}
	parts = append(parts, latest)
	return GenerateResponse{
		Provider:   c.Provider(),
		Content:    []ContentBlock{TextBlock(strings.Join(parts, " "))},
		StopReason: StopReasonEndTurn,
	}, nil
}
