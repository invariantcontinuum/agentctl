package agentsdk

import (
	"encoding/json"
	"net/http"
	"time"
)

// defaultHTTPClient returns an http.Client with a generous timeout suitable
// for chat-style calls. ModelClient implementations accept a custom
// HTTPClient for tests.
func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

// defaultJSONObject returns raw if non-empty, otherwise the literal "{}" so
// providers that require a JSON object never receive nil.
func defaultJSONObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

// maxTokensOrDefault returns value when positive, otherwise fallback.
func maxTokensOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
