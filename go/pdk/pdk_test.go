package pdk

import (
	"encoding/json"
	"os"
	"testing"
)

func TestExampleManifestIsValid(t *testing.T) {
	data, err := os.ReadFile("testdata/ups.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "de.example.ups" || len(m.Contributes.Kinds) != 1 {
		t.Fatalf("unexpected manifest %+v", m)
	}
}

func TestValidateRejects(t *testing.T) {
	base := Manifest{ID: "de.example.ups", Name: "USV", Version: "1.2.0", HostAPI: "1", Engines: Engines{Phoedox: ">=1.0.0"}}
	for name, mod := range map[string]func(*Manifest){
		"bad id":          func(m *Manifest) { m.ID = "ups" },
		"bad version":     func(m *Manifest) { m.Version = "1.2" },
		"unknown perm":    func(m *Manifest) { m.Permissions = []string{"fs:write"} },
		"too much memory": func(m *Manifest) { m.Backend = &Backend{Wasm: "a.wasm", MemoryMB: 1024} },
	} {
		m := base
		mod(&m)
		if m.Validate() == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("base manifest should be valid: %v", err)
	}
}

func TestGraphOpsRefs(t *testing.T) {
	var g GraphOps
	rack := g.Upsert("rack", "", map[string]any{"name": "Keller"})
	g.Link("01JABC", "mounted_in", rack, map[string]any{"pos": 4})
	b, _ := json.Marshal(g)
	want := `{"ops":[{"op":"upsert","ref":"$0","kind":"rack","attrs":{"name":"Keller"}},{"op":"link","from":"01JABC","rel":"mounted_in","to":"$0","attrs":{"pos":4}}]}`
	if string(b) != want {
		t.Fatalf("got %s", b)
	}
}

func TestManifestRules(t *testing.T) {
	base := Manifest{ID: "de.example.ups", Name: "USV", Version: "1.2.0", HostAPI: "1", Engines: Engines{Phoedox: ">=1.0.0 <2.0.0"}}
	for name, mod := range map[string]func(*Manifest){
		"bad range": func(m *Manifest) { m.Engines.Phoedox = "latest" },
		"bad kind":  func(m *Manifest) { m.Contributes.Kinds = []Kind{{ID: "UPS"}} },
		"bad tool":  func(m *Manifest) { m.Contributes.MCPTools = []MCPTool{{Name: "Run Now"}} },
		"long tool": func(m *Manifest) { m.Contributes.MCPTools = []MCPTool{{Name: "x", TimeoutSec: 120}} },
		"bad event": func(m *Manifest) { m.Contributes.Events = []string{"cron:*"} },
		"target without control": func(m *Manifest) {
			m.Contributes.MCPTools = []MCPTool{{Name: "x", Target: &ToolTarget{Arg: "entity_id"}}}
		},
		"bad target": func(m *Manifest) {
			m.Contributes.MCPTools = []MCPTool{{Name: "x", Control: true, Target: &ToolTarget{Arg: "Entity-ID"}}}
		},
	} {
		m := base
		mod(&m)
		if m.Validate() == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	base.Contributes.Events = []string{"schedule:every=60s", "entity.*"}
	base.Contributes.MCPTools = []MCPTool{{Name: "x", Control: true, Target: &ToolTarget{Arg: "entity_id", Attr: "ha_entity"}}}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeResponse(t *testing.T) {
	var d EntityDetail
	if err := decodeResponse([]byte(`{"ok":true,"result":{"entity":{"slug":"usv","attrs":{"ladung":80}}}}`), &d); err != nil || d.Entity.Slug != "usv" {
		t.Fatalf("%v %+v", err, d)
	}
	err := decodeResponse([]byte(`{"ok":false,"error":"graph:write:server","code":"denied"}`), nil)
	if !IsCode(err, "denied") {
		t.Fatalf("%v", err)
	}
	var g GraphOps
	ref := g.UpsertNamed("ups", "usv", "USV", map[string]any{"ladung": 1})
	g.Link(ref, "powers", "nas", nil)
	b, _ := json.Marshal(g)
	if string(b) != `{"ops":[{"op":"upsert","ref":"$0","kind":"ups","id":"usv","name":"USV","attrs":{"ladung":1}},{"op":"link","from":"$0","rel":"powers","to":"nas"}]}` {
		t.Fatal(string(b))
	}
}
