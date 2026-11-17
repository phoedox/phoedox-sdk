package pdk

import "testing"

func TestManifestTypes(t *testing.T) {
	if _, err := ParseManifest([]byte(`{"type":"theme","id":"de.example.forest","name":"Forest","version":"1.0.0","engines":{"phoedox":">=1.0.0"}}`)); err != nil {
		t.Fatalf("theme without hostApi: %v", err)
	}
	if _, err := ParseManifest([]byte(`{"id":"de.example.x","name":"X","version":"1.0.0","engines":{"phoedox":">=1.0.0"}}`)); err == nil {
		t.Fatal("plugins need hostApi")
	}
	if _, err := ParseManifest([]byte(`{"type":"app","id":"de.example.x","name":"X","version":"1.0.0","engines":{"phoedox":">=1.0.0"},"permissions":["graph:read"]}`)); err == nil {
		t.Fatal("app templates have no permissions")
	}
	if _, err := ParseManifest([]byte(`{"type":"widget","id":"de.example.x","name":"X","version":"1.0.0","engines":{"phoedox":">=1.0.0"}}`)); err == nil {
		t.Fatal("unknown type")
	}
}

func TestManifestBlocks(t *testing.T) {
	base := `{"id":"com.phoedox.gantt","name":"Gantt","version":"1.0.0","hostApi":"1","engines":{"phoedox":">=1.0.0"},`
	ok := base + `"backend":{"wasm":"backend.wasm"},"frontend":{"module":"frontend.js"},"contributes":{"blocks":[{"id":"gantt","label":"Schedule","icon":"chart-gantt","element":"phx-gantt","board":true}]}}`
	m, err := ParseManifest([]byte(ok))
	if err != nil || len(m.Contributes.Blocks) != 1 || !m.Contributes.Blocks[0].Board {
		t.Fatalf("block manifest: %v %+v", err, m.Contributes)
	}
	for name, c := range map[string]string{
		"no backend":       base + `"contributes":{"blocks":[{"id":"gantt","label":"Schedule"}]}}`,
		"bad id":           base + `"backend":{"wasm":"b.wasm"},"contributes":{"blocks":[{"id":"Gantt-1","label":"Schedule"}]}}`,
		"no label":         base + `"backend":{"wasm":"b.wasm"},"contributes":{"blocks":[{"id":"gantt","label":" "}]}}`,
		"element, no ui":   base + `"backend":{"wasm":"b.wasm"},"contributes":{"blocks":[{"id":"gantt","label":"S","element":"phx-gantt"}]}}`,
		"element not phx-": base + `"backend":{"wasm":"b.wasm"},"frontend":{"module":"f.js"},"contributes":{"blocks":[{"id":"gantt","label":"S","element":"gantt-view"}]}}`,
	} {
		if _, err := ParseManifest([]byte(c)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
