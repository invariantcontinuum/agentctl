package agentsdk

import "fmt"

// HTTPError is the typed error every provider client returns when the
// provider's HTTP endpoint replies with a non-2xx status. Callers can
// `errors.As` to inspect the status code or the upstream body.
type HTTPError struct {
	Provider   string
	StatusCode int
	Body       string
}

// Error implements error.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: %d: %s", e.Provider, e.StatusCode, e.Body)
}
