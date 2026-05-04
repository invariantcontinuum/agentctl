package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/invariantcontinuum/agentctl/internal/credentials"
	"github.com/invariantcontinuum/agentctl/internal/model"
)

// modelProvider routes 'agentctl model <provider> auth ...' (and the few
// reserved subcommands above it) to the right handler. The shape is
// deliberately Docker-like:
//
//	agentctl model anthropic auth login
//	agentctl model openai     auth login --api-key sk-...
//	agentctl model vllm       auth login --endpoint http://localhost:8000/v1
//	agentctl model anthropic  auth logout
//	agentctl model anthropic  auth status
func (a *App) modelProvider(provider string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentctl model %s <auth> [...]", provider)
	}
	switch args[0] {
	case "auth":
		return a.modelAuth(provider, args[1:])
	}
	return fmt.Errorf("unknown model %s subcommand %q", provider, args[0])
}

func (a *App) modelAuth(provider string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentctl model %s auth <login|logout|status|ls>", provider)
	}
	switch args[0] {
	case "login":
		return a.modelAuthLogin(provider, args[1:])
	case "logout":
		return a.modelAuthLogout(provider, args[1:])
	case "status":
		return a.modelAuthStatus(provider, args[1:])
	case "ls":
		return a.modelAuthList()
	}
	return fmt.Errorf("unknown auth subcommand %q", args[0])
}

func (a *App) modelAuthLogin(provider string, args []string) error {
	flags := flag.NewFlagSet("auth login", flag.ContinueOnError)
	flags.SetOutput(a.errOut)
	apiKey := flags.String("api-key", "", "API key (skips interactive prompt)")
	endpoint := flags.String("endpoint", "", "endpoint URL (overrides catalog default)")
	noInteractive := flags.Bool("no-interactive", false, "fail if interactive input would be required")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("auth login does not accept positional arguments")
	}

	defaults := model.DefaultCatalog().Default(provider)

	endpointValue := *endpoint
	if endpointValue == "" {
		endpointValue = defaults.Endpoint
	}
	apiKeyValue := *apiKey
	authMode := defaults.Auth

	if authMode == "none" {
		if endpointValue == "" {
			endpointValue = defaults.Endpoint
		}
	}

	// Interactive prompts only fire when a value is missing AND the user
	// hasn't asked for non-interactive mode. Local providers like vllm and
	// llamacpp have auth=none so they only need an endpoint.
	if endpointValue == "" {
		if *noInteractive {
			return errors.New("endpoint is required (set --endpoint or use interactive login)")
		}
		prompted, err := promptValue(a.out, a.in(), fmt.Sprintf("Endpoint [%s]: ", defaults.Endpoint))
		if err != nil {
			return err
		}
		if prompted == "" {
			prompted = defaults.Endpoint
		}
		endpointValue = prompted
	}
	if authMode != "none" && apiKeyValue == "" {
		if *noInteractive {
			return errors.New("api-key is required (set --api-key or use interactive login)")
		}
		prompted, err := promptSecret(a.out, a.in(), fmt.Sprintf("%s API key: ", strings.ToUpper(provider)))
		if err != nil {
			return err
		}
		if prompted == "" {
			return errors.New("api-key cannot be empty")
		}
		apiKeyValue = prompted
	}

	creds := credentials.ProviderCredentials{
		APIKey:   apiKeyValue,
		Endpoint: endpointValue,
	}
	if defaults.CredentialEnv != "" {
		creds.ExtraEnv = map[string]string{defaults.CredentialEnv: apiKeyValue}
	}
	if err := a.credentials.Set(provider, creds); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "saved %s credentials to %s\n", provider, a.credentialsPath())
	return nil
}

func (a *App) modelAuthLogout(provider string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("auth logout does not accept arguments")
	}
	if err := a.credentials.Delete(provider); err != nil {
		if errors.Is(err, credentials.ErrNotFound) {
			fmt.Fprintf(a.out, "%s was not logged in\n", provider)
			return nil
		}
		return err
	}
	fmt.Fprintf(a.out, "removed %s credentials\n", provider)
	return nil
}

func (a *App) modelAuthStatus(provider string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("auth status does not accept arguments")
	}
	creds, ok, err := a.credentials.Get(provider)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(a.out, "%s\tnot logged in\n", provider)
		return nil
	}
	fmt.Fprintf(a.out, "%s\tlogged in\tendpoint=%s\tapi_key=%s\n",
		provider,
		displayValue(creds.Endpoint),
		maskKey(creds.APIKey),
	)
	return nil
}

func (a *App) modelAuthList() error {
	names, err := a.credentials.List()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Fprintln(a.out, "no providers logged in")
		return nil
	}
	for _, name := range names {
		creds, _, err := a.credentials.Get(name)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.out, "%s\t%s\t%s\n", name, displayValue(creds.Endpoint), maskKey(creds.APIKey))
	}
	return nil
}

func promptValue(out io.Writer, in io.Reader, prompt string) (string, error) {
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return "", err
	}
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(strings.TrimRight(line, "\n"), "\r"), nil
}

// promptSecret reads a secret from in. It uses POSIX `stty -echo` when both
// /dev/tty is writable and `stty` is on PATH, falling back to visible echo
// otherwise. The fallback is deliberate: agentctl prefers --api-key /
// stdin-piping for non-interactive flows, and pulling in a third-party
// terminal package only to mask one prompt is heavier than the threat model
// warrants.
func promptSecret(out io.Writer, in io.Reader, prompt string) (string, error) {
	muted, restore := tryMuteTerminal()
	if muted {
		defer restore()
	} else {
		fmt.Fprintln(out, "(secret input will be visible — use --api-key for non-interactive flows)")
	}
	value, err := promptValue(out, in, prompt)
	if err != nil {
		return "", err
	}
	if muted {
		fmt.Fprintln(out)
	}
	return value, nil
}

func maskKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if len(value) <= 6 {
		return strings.Repeat("*", len(value))
	}
	return value[:3] + strings.Repeat("*", len(value)-6) + value[len(value)-3:]
}

// in returns the configured stdin reader, defaulting to os.Stdin so callers
// can override for tests.
func (a *App) in() io.Reader {
	if a.stdin != nil {
		return a.stdin
	}
	return os.Stdin
}

func (a *App) credentialsPath() string {
	if store, ok := a.credentials.(*credentials.JSONStore); ok {
		return store.Path()
	}
	return "(in-memory)"
}
