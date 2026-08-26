package metamodel

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash"
)

// RenderProjection is the render-relevant slice of a metamodel: exactly the
// schema facts needed to render or diff a stored entity version faithfully —
// property definitions, display configuration, and the enum value lists a
// property may reference. It deliberately EXCLUDES the churny, non-render parts
// of the metamodel (automations, validations, cascade rules, colors, id
// config), so its content hash stays stable across edits that don't change how
// an entity renders.
//
// This is the unit the pgstore versioning feature (TKT-9INY0Y) content-addresses
// into schema_versions: an entity_version row references the hash of the
// projection in force when it was captured, so a historical version renders
// against the schema it was created under, not today's (possibly drifted) one.
//
// Projecting to render-relevant fields — rather than hashing the whole
// metamodel — is a dedup-correctness win: hashing the full schema would churn
// the hash on every automation/validation edit, forcing a new schema_versions
// row (and a new pointer on every subsequent version) even though nothing
// render-relevant changed.
type RenderProjection struct {
	// Entities maps each entity type name to its render projection. All types
	// are included (not just the version's own type) so a diff/timeline can
	// render related-entity titles as-of.
	Entities map[string]EntityProjection `json:"entities"`
	// Types maps each custom (enum) type name to its ordered value list — the
	// values a property of that type may take, needed to render/validate an
	// enum value in a historical snapshot.
	Types map[string][]string `json:"types"`
}

// EntityProjection is the render-relevant projection of one entity type.
type EntityProjection struct {
	// DisplayProperty is the explicit display-name property (may be empty, in
	// which case rendering falls back to the title/name/label autoderivation).
	DisplayProperty string `json:"display_property,omitempty"`
	// PropertyOrder is the property order as defined in the metamodel YAML,
	// preserved because display and diff rendering present properties in order.
	PropertyOrder []string `json:"property_order,omitempty"`
	// Properties maps each property name to its render-relevant definition.
	Properties map[string]PropertyProjection `json:"properties"`
}

// PropertyProjection is the render-relevant projection of one property
// definition: the fields that affect how a value is titled, typed, stringified,
// or diffed. Attachment/scan/transform config, description, and default are
// omitted — they don't change how a stored value renders.
type PropertyProjection struct {
	Type     string `json:"type"`
	Required bool   `json:"required"`
	List     bool   `json:"list"`
	Format   string `json:"format,omitempty"`
	// Values is the inline enum value list (for properties whose type is an
	// inline enum rather than a named custom type).
	Values []string `json:"values,omitempty"`
}

// RenderProjection returns the render-relevant projection of the metamodel.
// The result is deterministic: map iteration is never observed (all output is
// keyed maps and the hash sorts keys), so two calls on an equal metamodel
// produce an equal projection and hash.
func (m *Metamodel) RenderProjection() RenderProjection {
	proj := RenderProjection{
		Entities: make(map[string]EntityProjection, len(m.Entities)),
		Types:    make(map[string][]string, len(m.Types)),
	}
	for name, def := range m.Entities {
		ep := EntityProjection{
			DisplayProperty: def.DisplayProperty,
			Properties:      make(map[string]PropertyProjection, len(def.Properties)),
		}
		if len(def.PropertyOrder) > 0 {
			ep.PropertyOrder = append([]string(nil), def.PropertyOrder...)
		}
		for pname, pdef := range def.Properties {
			pp := PropertyProjection{
				Type:     pdef.Type,
				Required: pdef.Required,
				List:     pdef.List,
				Format:   pdef.Format,
			}
			if len(pdef.Values) > 0 {
				pp.Values = append([]string(nil), pdef.Values...)
			}
			ep.Properties[pname] = pp
		}
		proj.Entities[name] = ep
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

// Hash returns the content-address of the projection: a hex-encoded SHA-256
// over a length-prefixed, key-sorted encoding of every field. Length prefixes
// make the encoding unambiguous (a value cannot smuggle a delimiter to forge a
// different structure with the same bytes — the same defense internal/canonical
// uses for entity hashes), and sorting every map key makes the digest
// independent of Go map iteration order.
func (p RenderProjection) Hash() string {
	h := newProjectionHasher()
	h.tag('P')

	h.str("entities")
	h.count(len(p.Entities))
	for _, name := range sortedKeys(p.Entities) {
		ep := p.Entities[name]
		h.str(name)
		h.str("display")
		h.str(ep.DisplayProperty)
		h.str("order")
		h.count(len(ep.PropertyOrder))
		for _, k := range ep.PropertyOrder {
			h.str(k)
		}
		h.str("props")
		h.count(len(ep.Properties))
		for _, pname := range sortedKeys(ep.Properties) {
			pp := ep.Properties[pname]
			h.str(pname)
			h.str(pp.Type)
			h.boolean(pp.Required)
			h.boolean(pp.List)
			h.str(pp.Format)
			h.strList(pp.Values)
		}
	}

	h.str("types")
	h.count(len(p.Types))
	for _, name := range sortedKeys(p.Types) {
		h.str(name)
		h.strList(p.Types[name])
	}

	return h.sum()
}

// JSON returns the projection serialized as deterministic JSON, for storage in
// schema_versions.projection. The bytes are content-addressed by [RenderProjection.Hash] (a
// separate length-prefixed digest), so this serialization is for storage and
// re-render, not identity — encoding/json with sorted map keys is sufficient.
//
// It returns an error rather than panicking because it is called on the write
// path (the entitymanager version hook), where the contract is that versioning
// must never fail a write — the caller logs and swallows. In practice
// RenderProjection holds only strings, bools, and slices/maps of them, so an
// error is not reachable short of a runtime bug.
func (p RenderProjection) JSON() ([]byte, error) {
	return json.Marshal(p)
}

// projectionHasher streams a length-prefixed encoding into a SHA-256 hash. It
// mirrors internal/canonical's writer discipline (fixed-width length prefixes,
// framed counts, a leading tag byte) so the projection digest has the same
// collision resistance and determinism guarantees.
type projectionHasher struct {
	h hash.Hash
}

func newProjectionHasher() *projectionHasher { return &projectionHasher{h: sha256.New()} }

func (w *projectionHasher) tag(b byte) { w.h.Write([]byte{b}) }

func (w *projectionHasher) str(s string) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(s)))
	w.h.Write(n[:])
	w.h.Write([]byte(s))
}

// count writes a fixed-width element count, framing variable-length sequences.
func (w *projectionHasher) count(n int) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(n)) //nolint:gosec // n is always a len(), never negative
	w.h.Write(b[:])
}

func (w *projectionHasher) boolean(v bool) {
	if v {
		w.h.Write([]byte{1})
	} else {
		w.h.Write([]byte{0})
	}
}

func (w *projectionHasher) strList(vs []string) {
	w.count(len(vs))
	for _, v := range vs {
		w.str(v)
	}
}

func (w *projectionHasher) sum() string {
	return hex.EncodeToString(w.h.Sum(nil))
}
