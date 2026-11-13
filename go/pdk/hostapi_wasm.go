//go:build wasip1

package pdk

import (
	"encoding/json"
	"fmt"

	extism "github.com/extism/go-pdk"
)

// The host offers one function, phx_host(request) -> response, with JSON
// {"op": …, "args": …} -> {"ok": …, "result": …, "error": …, "code": …}.
// One dispatching function keeps the ABI stable while operations are added
// (HostAPI "1"). Every operation is checked against the manifest and runs
// with the rights of the user the plugin works for.
//
//go:wasmimport extism:host/user phx_host
func phxHost(uint64) uint64

func call(op string, args, out any) error {
	b, err := json.Marshal(hostRequest{Op: op, Args: args})
	if err != nil {
		return err
	}
	mem := extism.AllocateBytes(b)
	defer mem.Free()
	res := extism.FindMemory(phxHost(mem.Offset()))
	defer res.Free()
	return decodeResponse(res.ReadBytes(), out)
}

// Input decodes the call's JSON input.
func Input[T any]() (T, error) {
	var v T
	err := json.Unmarshal(extism.Input(), &v)
	return v, err
}

// Output sets the call's JSON output.
func Output(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	extism.Output(b)
	return nil
}

// Fail reports an error to the host and returns the exit code for exports.
func Fail(err error) int32 {
	extism.SetErrorString(err.Error())
	return 1
}

// Log writes to the plugin log admins see.
func Log(msg string) { extism.Log(extism.LogInfo, msg) }

// Logf formats and logs.
func Logf(format string, args ...any) { Log(fmt.Sprintf(format, args...)) }

// Warn logs a warning.
func Warn(msg string) { extism.Log(extism.LogWarn, msg) }

// HandleEvent runs fn for on_event:
//
//	//go:wasmexport on_event
//	func onEvent() int32 { return pdk.HandleEvent(handle) }
func HandleEvent(fn func(Event) error) int32 {
	ev, err := Input[Event]()
	if err == nil {
		err = fn(ev)
	}
	if err != nil {
		return Fail(err)
	}
	return 0
}

// HandleTool runs fn for tool and returns its result as output.
func HandleTool(fn func(ToolCall) (any, error)) int32 {
	tc, err := Input[ToolCall]()
	var out any
	if err == nil {
		out, err = fn(tc)
	}
	if err == nil {
		err = Output(out)
	}
	if err != nil {
		return Fail(err)
	}
	return 0
}

// Secret reads a secret the project owner set (needs secrets:<name>).
func Secret(name string) (string, error) {
	var v string
	err := call("secret.get", map[string]string{"name": name}, &v)
	return v, err
}

// Fetch makes an HTTP request (needs net:fetch:<host>).
func Fetch(req FetchRequest) (FetchResponse, error) {
	var res FetchResponse
	err := call("http.fetch", req, &res)
	return res, err
}

type graphAPI struct{}

// Graph reads and changes the knowledge graph of the project.
var Graph graphAPI

// Get returns an entity by slug or id (needs graph:read).
func (graphAPI) Get(ref string) (EntityDetail, error) {
	var d EntityDetail
	err := call("graph.get", map[string]string{"ref": ref}, &d)
	return d, err
}

// Query lists entities (needs graph:read).
func (graphAPI) Query(q Query) ([]Entity, error) {
	var es []Entity
	err := call("graph.query", q, &es)
	return es, err
}

// Apply applies changes (needs graph:write:<kinds> for every changed kind).
func (graphAPI) Apply(ops *GraphOps) (ApplyResult, error) {
	var r ApplyResult
	err := call("graph.apply", ops, &r)
	return r, err
}

type kvAPI struct{}

// KV is a small key-value store per plugin and project (1000 keys, 256 KB
// per value).
var KV kvAPI

// Get reads a value into out; found is false for missing keys.
func (kvAPI) Get(key string, out any) (found bool, err error) {
	var raw json.RawMessage
	if err := call("kv.get", map[string]string{"key": key}, &raw); err != nil {
		return false, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return false, nil
	}
	return true, json.Unmarshal(raw, out)
}

// Set writes a value; nil deletes the key.
func (kvAPI) Set(key string, value any) error {
	return call("kv.set", map[string]any{"key": key, "value": value}, nil)
}
