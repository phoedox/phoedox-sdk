// Package pdk is the Go plugin development kit for Phoedox backend plugins.
// Plugins are compiled to WASM (GOOS=wasip1 or TinyGo) and run in the
// Phoedox host via Extism. The host API itself is in hostapi_wasm.go.
package pdk

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// HostAPI is the host API major version this SDK targets.
const HostAPI = "1"

// Manifest mirrors schema/phoedox-plugin.schema.json.
type Manifest struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	HostAPI     string  `json:"hostApi"`
	Engines     Engines `json:"engines"`
	Description string  `json:"description,omitempty"`
	// Icon is a Lucide icon name (kebab-case, e.g. battery-charging) shown in the store and the instance.
	Icon string `json:"icon,omitempty"`
	// Screenshots are images in the package (screenshots/…, PNG, JPEG or WebP) shown in the store.
	Screenshots  []Screenshot `json:"screenshots,omitempty"`
	Backend      *Backend     `json:"backend,omitempty"`
	Frontend     *Frontend    `json:"frontend,omitempty"`
	CriticalPath bool         `json:"criticalPath,omitempty"`
	Contributes  Contributes  `json:"contributes,omitempty"`
	// Permissions: net:fetch:<host> for a fixed host, net:fetch:config for
	// the hosts of the addresses the project owner sets (secrets url, *_url).
	Permissions []string        `json:"permissions,omitempty"`
	License     json.RawMessage `json:"license,omitempty"`
}

// Screenshot is an image of the plugin in use.
type Screenshot struct {
	File    string `json:"file"`
	Caption string `json:"caption,omitempty"`
}

// MaxScreenshots limits the screenshots of a plugin.
const MaxScreenshots = 6

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
	// Control marks tools that act on the physical world (switching,
	// restarting). Only editors run them; AI calls become suggestions.
	Control bool `json:"control,omitempty"`
	// TimeoutSec limits one call (1–60 s, default 10).
	TimeoutSec int `json:"timeoutSec,omitempty"`
	// Target names the entry a control tool acts on (sharing single
	// devices, AI control per device).
	Target *ToolTarget `json:"target,omitempty"`
}

// ToolTarget points from a tool argument to an entry: without Attr the
// argument is the slug of an entry of a kind the plugin contributes, with
// Attr it matches that attribute of any entry.
type ToolTarget struct {
	Arg  string `json:"arg"`
	Attr string `json:"attr,omitempty"`
}

var (
	argRe     = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	idRe      = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9-]+)+$`)
	semverRe  = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	permRe    = regexp.MustCompile(`^(graph|docs|assets|events|ai|ha):(read|write|use|emit|control)(:[a-z0-9_,]+)?$|^net:fetch:[a-z0-9.-]+$|^secrets:[a-z0-9_]+$`)
	kindRe    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	toolRe    = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)
	eventRe   = regexp.MustCompile(`^(\*|[a-z_]+\.(\*|[a-z_]+))$|^schedule:every=\d+(s|m|h)$`)
	iconRe    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	shotRe    = regexp.MustCompile(`^screenshots/[A-Za-z0-9._-]+\.(png|jpe?g|webp)$`)
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
		if t.TimeoutSec < 0 || t.TimeoutSec > 60 {
			return fmt.Errorf("manifest: tool %s: timeoutSec must be between 1 and 60", t.Name)
		}
		if t.Target != nil && (!argRe.MatchString(t.Target.Arg) || (t.Target.Attr != "" && !argRe.MatchString(t.Target.Attr)) || !t.Control) {
			return fmt.Errorf("manifest: tool %s: target needs control and arg/attr of lowercase letters, digits and _", t.Name)
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
	if m.Icon != "" && (len(m.Icon) > 40 || !iconRe.MatchString(m.Icon)) {
		return fmt.Errorf("manifest: icon %q must be a Lucide name like battery-charging", m.Icon)
	}
	if len(m.Screenshots) > MaxScreenshots {
		return fmt.Errorf("manifest: at most %d screenshots", MaxScreenshots)
	}
	for _, sc := range m.Screenshots {
		if !shotRe.MatchString(sc.File) || strings.Contains(sc.File, "..") {
			return fmt.Errorf("manifest: screenshot %q must be a PNG, JPEG or WebP file in screenshots/", sc.File)
		}
		if utf8.RuneCountInString(sc.Caption) > 140 {
			return fmt.Errorf("manifest: screenshot caption of %s is longer than 140 characters", sc.File)
		}
	}
	for _, p := range m.Permissions {
		if !permRe.MatchString(p) {
			return fmt.Errorf("manifest: unknown permission %q", p)
		}
	}
	return nil
}
