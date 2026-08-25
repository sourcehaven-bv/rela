package metamodel

import "encoding/json"

// ShapeProjection is the data-shape slice of a metamodel: exactly the schema
// facts that determine whether STORED CONTENT conforms to the schema —
// entity properties (type, required, list, format, inline values, default),
// named enum value lists, and relation types (endpoints, cardinality,
// symmetry, content flag, relation properties).
//
// It is the identity unit of the data-migration system (TKT-0C57FS): the
// hash of the projection in force is the "schema version" a store's content
// conforms to, recorded in the state.KV marker and referenced by migration
// files. Cosmetic schema edits (labels, descriptions, colors, views,
// automations, validations, display configuration, id prefixes) do not move
// the hash, so they never demand a migration.
//
// This is a SIBLING of RenderProjection, not a replacement. The two answer
// different questions — "how does a stored version render?" versus "does the
// stored data still fit the schema?" — and have independent stability
// contracts: RenderProjection's hash content-addresses schema_versions rows
// for the pgstore versioning feature and must not churn, while
// ShapeProjection deliberately includes relation shape and property defaults
// that rendering never needs. Do not merge them.
//
// id prefixes are deliberately EXCLUDED (TKT-0C57FS amendment A5): no v1
// migration step can rewrite entity IDs, so including prefixes would let a
// prefix edit create a needs-migration state no migration could resolve.
type ShapeProjection struct {
	// Entities maps each entity type name to its property shapes.
	Entities map[string]EntityShape `json:"entities"`
	// Relations maps each relation type name to its shape.
	Relations map[string]RelationShape `json:"relations"`
	// Types maps each named custom (enum) type to its ordered value list.
	Types map[string][]string `json:"types"`
}

// EntityShape is the data-shape projection of one entity type.
type EntityShape struct {
	Properties map[string]PropertyShape `json:"properties"`
}

// PropertyShape is the data-shape projection of one property definition:
// the fields that determine whether a stored value conforms and how it would
// be coerced. Display labels, descriptions, uniqueness and attachment/scan
// config are omitted — they constrain writes or presentation, not the shape
// of values already stored.
type PropertyShape struct {
	Type     string   `json:"type"`
	Required bool     `json:"required,omitempty"`
	List     bool     `json:"list,omitempty"`
	Format   string   `json:"format,omitempty"`
	Values   []string `json:"values,omitempty"` // inline enum value list
	// Default is included even though it only affects future creates:
	// the generator offers backfill steps from it, and the classifier
	// tiers default-only changes additive (amendment A7).
	Default string `json:"default,omitempty"`
}

// RelationShape is the data-shape projection of one relation type.
type RelationShape struct {
	From        []string                 `json:"from,omitempty"`
	To          []string                 `json:"to,omitempty"`
	Symmetric   bool                     `json:"symmetric,omitempty"`
	MinOutgoing *int                     `json:"min_outgoing,omitempty"`
	MaxOutgoing *int                     `json:"max_outgoing,omitempty"`
	MinIncoming *int                     `json:"min_incoming,omitempty"`
	MaxIncoming *int                     `json:"max_incoming,omitempty"`
	Content     bool                     `json:"content,omitempty"`
	Properties  map[string]PropertyShape `json:"properties,omitempty"`
}

// ShapeProjection returns the data-shape projection of the metamodel.
// The result is deterministic: all output is keyed maps plus slices copied
// in declaration order, and the hash sorts every map key.
func (m *Metamodel) ShapeProjection() ShapeProjection {
	proj := ShapeProjection{
		Entities:  make(map[string]EntityShape, len(m.Entities)),
		Relations: make(map[string]RelationShape, len(m.Relations)),
		Types:     make(map[string][]string, len(m.Types)),
	}
	for name, def := range m.Entities {
		es := EntityShape{Properties: make(map[string]PropertyShape, len(def.Properties))}
		for pname, pdef := range def.Properties {
			es.Properties[pname] = propertyShape(pdef)
		}
		proj.Entities[name] = es
	}
	for name, def := range m.Relations {
		rs := RelationShape{
			From:        append([]string(nil), def.From...),
			To:          append([]string(nil), def.To...),
			Symmetric:   def.Symmetric,
			MinOutgoing: cloneIntPtr(def.MinOutgoing),
			MaxOutgoing: cloneIntPtr(def.MaxOutgoing),
			MinIncoming: cloneIntPtr(def.MinIncoming),
			MaxIncoming: cloneIntPtr(def.MaxIncoming),
			Content:     def.Content,
		}
		if len(def.Properties) > 0 {
			rs.Properties = make(map[string]PropertyShape, len(def.Properties))
			for pname, pdef := range def.Properties {
				rs.Properties[pname] = propertyShape(pdef)
			}
		}
		proj.Relations[name] = rs
	}
	for name, ct := range m.Types {
		if len(ct.Values) > 0 {
			proj.Types[name] = append([]string(nil), ct.Values...)
		} else {
			proj.Types[name] = nil
		}
	}
	return proj
}

func propertyShape(p PropertyDef) PropertyShape {
	ps := PropertyShape{
		Type:     p.Type,
		Required: p.Required,
		List:     p.List,
		Format:   p.Format,
		Default:  p.Default,
	}
	if len(p.Values) > 0 {
		ps.Values = append([]string(nil), p.Values...)
	}
	return ps
}

func cloneIntPtr(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// Hash returns the content-address of the shape projection: a hex-encoded
// SHA-256 over a length-prefixed, key-sorted encoding of every field — the
// same writer discipline as RenderProjection.Hash and internal/canonical
// (length prefixes make the encoding unambiguous; sorted keys make it
// independent of map iteration order). The leading tag byte differs ('S'
// versus RenderProjection's 'P') so the two hash spaces can never collide
// even on structurally similar input.
func (p ShapeProjection) Hash() string {
	h := newProjectionHasher()
	h.tag('S')

	h.str("entities")
	h.count(len(p.Entities))
	for _, name := range sortedKeys(p.Entities) {
		h.str(name)
		hashPropertyShapes(h, p.Entities[name].Properties)
	}

	h.str("relations")
	h.count(len(p.Relations))
	for _, name := range sortedKeys(p.Relations) {
		rs := p.Relations[name]
		h.str(name)
		h.strList(rs.From)
		h.strList(rs.To)
		h.boolean(rs.Symmetric)
		h.optInt(rs.MinOutgoing)
		h.optInt(rs.MaxOutgoing)
		h.optInt(rs.MinIncoming)
		h.optInt(rs.MaxIncoming)
		h.boolean(rs.Content)
		hashPropertyShapes(h, rs.Properties)
	}

	h.str("types")
	h.count(len(p.Types))
	for _, name := range sortedKeys(p.Types) {
		h.str(name)
		h.strList(p.Types[name])
	}

	return h.sum()
}

func hashPropertyShapes(h *projectionHasher, props map[string]PropertyShape) {
	h.count(len(props))
	for _, pname := range sortedKeys(props) {
		ps := props[pname]
		h.str(pname)
		h.str(ps.Type)
		h.boolean(ps.Required)
		h.boolean(ps.List)
		h.str(ps.Format)
		h.strList(ps.Values)
		h.str(ps.Default)
	}
}

// optInt writes an optional int as a presence byte plus (when present) a
// fixed-width value, keeping absent and zero distinguishable.
func (w *projectionHasher) optInt(p *int) {
	if p == nil {
		w.boolean(false)
		return
	}
	w.boolean(true)
	w.count(*p)
}

// JSON returns the projection serialized as deterministic JSON, for embedding
// in migration files and the state.KV marker. Identity is [ShapeProjection.Hash] (a separate
// length-prefixed digest); this serialization is for storage and re-load.
func (p ShapeProjection) JSON() ([]byte, error) {
	return json.Marshal(p)
}

// ShapeProjectionFromJSON parses a projection previously serialized by
// [ShapeProjection.JSON] (from a migration file or the state marker).
func ShapeProjectionFromJSON(data []byte) (ShapeProjection, error) {
	var p ShapeProjection
	if err := json.Unmarshal(data, &p); err != nil {
		return ShapeProjection{}, err
	}
	return p, nil
}
