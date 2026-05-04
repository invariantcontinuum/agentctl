package agent

import "testing"

func TestEndpointRoundTripURL(t *testing.T) {
	endpoint, err := EndpointFromURL("http", "https://example.org:7000/api")
	if err != nil {
		t.Fatalf("EndpointFromURL returned error: %v", err)
	}
	if endpoint.Name != "http" || endpoint.Scheme != "https" || endpoint.Host != "example.org" || endpoint.Port != 7000 || endpoint.Path != "/api" {
		t.Fatalf("endpoint = %+v", endpoint)
	}
	if got := EndpointURL(endpoint); got != "https://example.org:7000/api" {
		t.Fatalf("EndpointURL = %q", got)
	}
	if got := EndpointHostPort(endpoint); got != "example.org:7000" {
		t.Fatalf("EndpointHostPort = %q", got)
	}
}
