//go:build otel

// The OpenTelemetry sink exists only in a binary built with `-tags otel`. It speaks OTLP over
// HTTP/JSON directly rather than through the OpenTelemetry Go SDK: the wire format for a log
// record is forty lines, and the alternative is a dependency tree in every default build for a
// sink almost nobody turns on. Build the exporter in when you want it:
//
//	go build -tags otel ./cmd/boxer
package obs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func init() { otelSink = send }

// send posts one event as an OTLP log record. Without an endpoint nothing is sent, which is the
// promise SECURITY.md makes: telemetry never leaves the machine unless you name where it goes.
func send(c Config, e Event) {
	if c.Endpoint == "" {
		return
	}
	url := strings.TrimSuffix(c.Endpoint, "/") + "/v1/logs"
	body, err := json.Marshal(payload(e))
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func payload(e Event) map[string]any {
	attrs := []map[string]any{
		{"key": "event.name", "value": map[string]any{"stringValue": e.Name}},
		{"key": "boxer.scope", "value": map[string]any{"stringValue": e.Scope}},
		{"key": "boxer.harness", "value": map[string]any{"stringValue": e.Harness}},
		{"key": "boxer.outcome", "value": map[string]any{"stringValue": e.Outcome}},
		{"key": "boxer.duration_ms", "value": map[string]any{"doubleValue": float64(e.Duration) / float64(time.Millisecond)}},
	}
	for k, v := range e.Payload {
		attrs = append(attrs, map[string]any{"key": "boxer." + k, "value": map[string]any{"stringValue": fmt.Sprint(v)}})
	}
	return map[string]any{"resourceLogs": []map[string]any{{
		"resource": map[string]any{"attributes": []map[string]any{
			{"key": "service.name", "value": map[string]any{"stringValue": "boxer"}},
		}},
		"scopeLogs": []map[string]any{{"logRecords": []map[string]any{{
			"timeUnixNano": fmt.Sprint(e.Time.UnixNano()),
			"body":         map[string]any{"stringValue": e.Name},
			"attributes":   attrs,
		}}}},
	}}}
}
