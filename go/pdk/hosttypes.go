package pdk

import (
	"encoding/json"
	"time"
)

// Types exchanged with the host API (phoedox: internal/plugins and
// internal/core). They are plain data and also compile outside WASM, so
// plugins can unit-test their logic natively.

// Entity is an entry of the knowledge graph.
type Entity struct {
	ID     string         `json:"id"`
	Kind   string         `json:"kind"`
	Slug   string         `json:"slug"`
	Name   string         `json:"name"`
	Attrs  map[string]any `json:"attrs"`
	Source string         `json:"source,omitempty"`
}

// Edge connects two entities.
type Edge struct {
	ID    string         `json:"id"`
	From  string         `json:"from"`
	Rel   string         `json:"rel"`
	To    string         `json:"to"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// EntityDetail is an entity with its edges.
type EntityDetail struct {
	Entity Entity `json:"entity"`
	Edges  []Edge `json:"edges"`
}

// Query searches entities of the project (name or slug contains Q).
type Query struct {
	Kind  string `json:"kind,omitempty"`
	Q     string `json:"q,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// ApplyResult tells what Graph.Apply did. Refs maps "$n" references to the
// slugs of upserted entities. In projects where AI clients only suggest,
// changes the plugin makes for an AI client become suggestions.
type ApplyResult struct {
	Applied     int               `json:"applied"`
	Refs        map[string]string `json:"refs"`
	Suggestions int               `json:"suggestions,omitempty"`
}

// FetchRequest is an HTTP request to a host listed as net:fetch:<host>.
// Redirects are not followed.
type FetchRequest struct {
	Method  string            `json:"method,omitempty"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// FetchResponse is the answer (headers in lower case).
type FetchResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
}

// Event is the input of on_event: "schedule" for timed runs or an event
// type like "entity.saved".
type Event struct {
	Type    string          `json:"type"`
	Project string          `json:"project"`
	Subject string          `json:"subject,omitempty"`
	Actor   string          `json:"actor,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
	At      time.Time       `json:"at"`
}

// ToolCall is the input of tool: a tool listed in contributes.mcpTools.
type ToolCall struct {
	Name    string          `json:"name"`
	Args    json.RawMessage `json:"args"`
	Project string          `json:"project"`
}

// HostError is an error the host returned. Code is one of denied (not in
// the manifest), forbidden (the user lacks the role), not_found, invalid,
// conflict, quota, failed.
type HostError struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

func (e *HostError) Error() string { return e.Code + ": " + e.Message }

// IsCode tells whether err is a HostError with the code.
func IsCode(err error, code string) bool {
	h, ok := err.(*HostError)
	return ok && h.Code == code
}

type hostRequest struct {
	Op   string `json:"op"`
	Args any    `json:"args"`
}

type hostResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
	Code   string          `json:"code"`
}

// decodeResponse turns a phx_host answer into out or a HostError.
func decodeResponse(raw []byte, out any) error {
	var r hostResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return &HostError{Code: "failed", Message: "bad host response"}
	}
	if !r.OK {
		return &HostError{Code: r.Code, Message: r.Error}
	}
	if out == nil || len(r.Result) == 0 || string(r.Result) == "null" {
		return nil
	}
	return json.Unmarshal(r.Result, out)
}
