// Package pdk is the Go plugin development kit for Phoedox backend plugins.
// Plugins are compiled to WASM (GOOS=wasip1 or TinyGo) and run in the
// Phoedox host via Extism. The host API itself is in hostapi_wasm.go.
package pdk

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// HostAPI is the host API major version this SDK targets.
const HostAPI = "1"

// Manifest mirrors schema/phoedox-plugin.schema.json.
type Manifest struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	HostAPI      string          `json:"hostApi"`
	Engines      Engines         `json:"engines"`
	Description  string          `json:"description,omitempty"`
	Backend      *Backend        `json:"backend,omitempty"`
	Frontend     *Frontend       `json:"frontend,omitempty"`
	CriticalPath bool            `json:"criticalPath,omitempty"`
	Contributes  Contributes     `json:"contributes,omitempty"`
	Permissions  []string        `json:"permissions,omitempty"`
	License      json.RawMessage `json:"license,omitempty"`
}

type Engines struct {
	Phoedox string `json:"phoedox"`
}

type Backend struct {
	Wasm     string `json:"wasm"`
	MemoryMB int    `json:"memoryMb,omitempty"`
}

type Frontend struct {
	Module string `json:"module"`
}

type Contributes struct {
	Kinds     []Kind     `json:"kinds,omitempty"`
	Relations []Relation `json:"relations,omitempty"`
	MCPTools  []MCPTool  `json:"mcpTools,omitempty"`
	Events    []string   `json:"events,omitempty"`
}

type Kind struct {
	ID     string `json:"id"`
	Schema string `json:"schema,omitempty"`
	Icon   string `json:"icon,omitempty"`
}

type Relation struct {
	ID   string   `json:"id"`
	From []string `json:"from"`
	To   []string `json:"to"`
	Card string   `json:"card,omitempty"`
}

type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Input       string `json:"input,omitempty"` // JSON Schema file of the arguments
}

var (
	idRe      = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9-]+)+$`)
	semverRe  = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	permRe    = regexp.MustCompile(`^(graph|docs|assets|events|ai|ha):(read|write|use|emit|control)(:[a-z0-9_,]+)?$|^net:fetch:[a-z0-9.-]+$|^secrets:[a-z0-9_]+$`)
	kindRe    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	toolRe    = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)
	eventRe   = regexp.MustCompile(`^(\*|[a-z_]+\.(\*|[a-z_]+))$|^schedule:every=\d+(s|m|h)$`)
	rangeRe   = regexp.MustCompile(`^((>=|<=|>|<|=)?\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?\s*)+$`)
	maxMemory = 256
)

// ParseManifest decodes and validates a manifest.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("manifest: %w", err)
	}
	return m, m.Validate()
}

// Validate checks the fields the host relies on.
func (m Manifest) Validate() error {
	switch {
	case !idRe.MatchString(m.ID):
		return fmt.Errorf("manifest: id %q must be reverse-DNS like de.example.ups", m.ID)
	case m.Name == "":
		return fmt.Errorf("manifest: name is required")
	case !semverRe.MatchString(m.Version):
		return fmt.Errorf("manifest: version %q is not SemVer", m.Version)
	case m.HostAPI == "":
		return fmt.Errorf("manifest: hostApi is required")
	case m.Engines.Phoedox == "":
		return fmt.Errorf("manifest: engines.phoedox is required")
	}
	if !rangeRe.MatchString(m.Engines.Phoedox) {
		return fmt.Errorf("manifest: engines.phoedox %q must be a range like >=1.4.0 <2.0.0", m.Engines.Phoedox)
	}
	for _, k := range m.Contributes.Kinds {
		if !kindRe.MatchString(k.ID) {
			return fmt.Errorf("manifest: kind %q must be lowercase letters, digits and _", k.ID)
		}
	}
	for _, t := range m.Contributes.MCPTools {
		if !toolRe.MatchString(t.Name) {
			return fmt.Errorf("manifest: tool name %q is invalid", t.Name)
		}
	}
	for _, e := range m.Contributes.Events {
		if !eventRe.MatchString(e) {
			return fmt.Errorf("manifest: event %q must be schedule:every=<n>s|m|h or a type like entity.saved", e)
		}
	}
	if m.Backend != nil && (m.Backend.MemoryMB < 0 || m.Backend.MemoryMB > maxMemory) {
		return fmt.Errorf("manifest: backend.memoryMb must be between 1 and %d", maxMemory)
	}
	for _, p := range m.Permissions {
		if !permRe.MatchString(p) {
			return fmt.Errorf("manifest: unknown permission %q", p)
		}
	}
	return nil
}
