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
