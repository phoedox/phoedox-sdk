package pdk

// GraphOps collects graph changes a plugin returns from extract/on_event.
// The host applies them atomically and checks permissions for each kind.
type GraphOps struct {
	Ops []GraphOp `json:"ops"`
}

// GraphOp is one change. Ref lets later ops point to entities created earlier
// in the same batch before they have a real ID.
type GraphOp struct {
	Op    string         `json:"op"` // upsert | link | unlink | delete
	Ref   string         `json:"ref,omitempty"`
	Kind  string         `json:"kind,omitempty"`
	ID    string         `json:"id,omitempty"`
	From  string         `json:"from,omitempty"`
	Rel   string         `json:"rel,omitempty"`
	To    string         `json:"to,omitempty"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Upsert creates or updates an entity and returns a reference for later ops.
func (g *GraphOps) Upsert(kind, id string, attrs map[string]any) string {
	ref := "$" + itoa(len(g.Ops))
	g.Ops = append(g.Ops, GraphOp{Op: "upsert", Ref: ref, Kind: kind, ID: id, Attrs: attrs})
	return ref
}

// Link adds an edge. from/to may be entity IDs or references from Upsert.
func (g *GraphOps) Link(from, rel, to string, attrs map[string]any) {
	g.Ops = append(g.Ops, GraphOp{Op: "link", From: from, Rel: rel, To: to, Attrs: attrs})
}

// Unlink removes an edge.
func (g *GraphOps) Unlink(from, rel, to string) {
	g.Ops = append(g.Ops, GraphOp{Op: "unlink", From: from, Rel: rel, To: to})
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}

// Document is the input of validate/extract for a plugin document type.
type Document[T any] struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	SchemaVersion int    `json:"schemaVersion"`
	Content       T      `json:"content"`
}
