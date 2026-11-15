package mapimport

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Dir is the project directory that holds mapping files.
const Dir = "imports"

// Record caps.
const (
	DefaultMaxRecords = 10_000
	MaxMaxRecords     = 100_000
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// ValidName reports whether name may name a mapping file or an entity
// mapping: lowercase letters, digits, `-` and `_`, at most 63 characters.
func ValidName(name string) bool { return namePattern.MatchString(name) }

// FilePath returns the project-relative path of the mapping called name.
func FilePath(name string) string { return Dir + "/" + name + ".yaml" }

// Mapping is one parsed and validated mapping file.
type Mapping struct {
	Name        string                    `yaml:"-"`
	Description string                    `yaml:"description,omitempty"`
	Format      Format                    `yaml:"format,omitempty"`
	MaxBytes    int                       `yaml:"max_bytes,omitempty"`
	MaxRecords  int                       `yaml:"max_records,omitempty"`
	Entities    map[string]*EntityMapping `yaml:"entities"`
}

// EntityMapping maps the records one path selects onto one entity type.
type EntityMapping struct {
	name string

	// Type is the entity type the records become. Faced types are refused.
	Type string `yaml:"type"`
	// Select is the path from the document root; each node it yields is one
	// record.
	Select string `yaml:"select"`
	// Key is the property that identifies an entity across runs. It must be
	// `unique:` or an `external_ref`, and Properties must map it.
	Key string `yaml:"key,omitempty"`
	// ID is an optional {{path}} template for the id of a new entity. With no
	// Key, it is also how existing entities are matched.
	ID string `yaml:"id,omitempty"`
	// Scope holds constant property values. Only stored entities with these
	// values are matched and reported missing, and new and updated entities
	// get them. Two imports that share a type keep apart this way.
	Scope map[string]string `yaml:"scope,omitempty"`
	// Properties maps property names to values derived from the record.
	Properties map[string]*Value `yaml:"properties,omitempty"`
	// Content, when set, makes the markdown body owned by the import.
	Content *Value `yaml:"content,omitempty"`
	// Relations derive relations from the record.
	Relations []*RelationMapping `yaml:"relations,omitempty"`
	// Hook is an optional Lua transform run per record.
	Hook *Hook `yaml:"hook,omitempty"`

	selectPath *Path
	idParts    *template
}

// Name returns the mapping's name in the file.
func (em *EntityMapping) Name() string { return em.name }

// Value derives one property value from a record. Exactly one of Path,
// Template and Const is set. A plain string in YAML is a Path.
type Value struct {
	Path     string `yaml:"path,omitempty"`
	Template string `yaml:"template,omitempty"`
	Const    any    `yaml:"value,omitempty"`
	// Join joins several matches into one string for a property that is not
	// a list. Without it, several matches are a record error.
	Join *string `yaml:"join,omitempty"`
	// ClearMissing removes the stored value when the source has none. Off by
	// default: a filtered export or a path that matches nothing would
	// otherwise wipe the property on every entity.
	ClearMissing bool `yaml:"clear_missing,omitempty"`

	path *Path
	tmpl *template
}

// UnmarshalYAML accepts a plain string as a path.
func (v *Value) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!str" {
		v.Path = n.Value
		return nil
	}
	type plain Value
	var p plain
	if err := decodeStrict(n, &p); err != nil {
		return err
	}
	*v = Value(p)
	return nil
}

// RelationMapping derives relations from a record.
type RelationMapping struct {
	// Name identifies the relation mapping in the hook's draft; it defaults
	// to Type and must be unique within the entity mapping.
	Name string `yaml:"name,omitempty"`
	// Type is the relation type.
	Type string `yaml:"type"`
	// Target names another entity mapping in this file; the key is matched
	// against that mapping's records and stored entities.
	Target string `yaml:"target,omitempty"`
	// TargetType and TargetKey match existing entities of a type this file
	// does not import, by a unique or external_ref property.
	TargetType string `yaml:"target_type,omitempty"`
	TargetKey  string `yaml:"target_key,omitempty"`
	// Key yields the target's key; several matches make several relations.
	Key *Value `yaml:"key"`
	// Direction is "out" (record → target, the default) or "in".
	Direction string `yaml:"direction,omitempty"`
	// Optional makes a record with no target key, or an unknown one, import
	// without the relation instead of being a record error.
	Optional bool `yaml:"optional,omitempty"`
	// Prune removes stored relations of this type from the record's entity
	// to entities of the target type that the source no longer names.
	Prune bool `yaml:"prune,omitempty"`
}

// Hook names a Lua function that transforms each record's draft.
type Hook struct {
	Script   string `yaml:"script"`
	Function string `yaml:"function,omitempty"`
}

// UnmarshalYAML accepts a plain string as the script path.
func (h *Hook) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		h.Script = n.Value
		return nil
	}
	type plain Hook
	var p plain
	if err := decodeStrict(n, &p); err != nil {
		return err
	}
	*h = Hook(p)
	return nil
}

// DefaultHookFunction is the hook function called when none is named.
const DefaultHookFunction = "transform"

func (h *Hook) function() string {
	if h.Function == "" {
		return DefaultHookFunction
	}
	return h.Function
}

// decodeStrict decodes n into out, refusing unknown fields. yaml.Node.Decode
// does not honor KnownFields, so the node is re-encoded and decoded.
func decodeStrict(n *yaml.Node, out any) error {
	data, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	return d.Decode(out)
}

// Names returns the entity mapping names in sorted order, the order in which
// a plan processes them.
func (m *Mapping) Names() []string {
	names := make([]string, 0, len(m.Entities))
	for n := range m.Entities {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (m *Mapping) maxBytes() int {
	if m.MaxBytes > 0 {
		return m.MaxBytes
	}
	return DefaultMaxBytes
}

// Limits returns the decode limits the mapping asks for.
func (m *Mapping) Limits() Limits { return Limits{MaxBytes: m.maxBytes()} }

func (m *Mapping) maxRecords() int {
	if m.MaxRecords > 0 {
		return m.MaxRecords
	}
	return DefaultMaxRecords
}

// Parse reads and validates the mapping file called name. Every problem is
// reported, one per line, prefixed with the file path.
func Parse(name string, data []byte, meta *metamodel.Metamodel) (*Mapping, error) {
	file := FilePath(name)
	if !ValidName(name) {
		return nil, fmt.Errorf("invalid import name %q: use lowercase letters, digits, - and _", name)
	}
	if meta == nil {
		return nil, errors.New("mapimport: a schema is required")
	}
	var m Mapping
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(&m); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	m.Name = name
	if errs := m.validate(meta); len(errs) > 0 {
		return nil, fmt.Errorf("%s: %s", file, strings.Join(errs, "; "))
	}
	return &m, nil
}

func (m *Mapping) validate(meta *metamodel.Metamodel) []string {
	var errs []string
	switch m.Format {
	case "", FormatJSON, FormatYAML, FormatXML, FormatCSV:
	default:
		errs = append(errs, fmt.Sprintf("format %q is not json, yaml, xml or csv", m.Format))
	}
	if m.MaxBytes < 0 || m.MaxBytes > MaxMaxBytes {
		errs = append(errs, fmt.Sprintf("max_bytes must be at most %d (0 or unset is the default)", MaxMaxBytes))
	}
	if m.MaxRecords < 0 || m.MaxRecords > MaxMaxRecords {
		errs = append(errs, fmt.Sprintf("max_records must be at most %d (0 or unset is the default)", MaxMaxRecords))
	}
	if len(m.Entities) == 0 {
		errs = append(errs, "entities: declares no mapping")
	}
	for _, name := range m.Names() {
		em := m.Entities[name]
		if em == nil {
			errs = append(errs, fmt.Sprintf("entities.%s: is empty", name))
			continue
		}
		em.name = name
		if !ValidName(name) {
			errs = append(errs, fmt.Sprintf("entities.%s: invalid name (use lowercase letters, digits, - and _)", name))
		}
		for _, e := range em.validate(m, meta) {
			errs = append(errs, fmt.Sprintf("entities.%s: %s", name, e))
		}
	}
	return append(errs, m.validateOverlap()...)
}

// validateOverlap refuses two entity mappings of one type whose scopes can
// hold the same entity. Each would report the other's entities as missing,
// and both could plan a create for one key.
func (m *Mapping) validateOverlap() []string {
	var errs []string
	names := m.Names()
	for i, a := range names {
		for _, b := range names[i+1:] {
			ea, eb := m.Entities[a], m.Entities[b]
			if ea == nil || eb == nil || ea.Type != eb.Type || scopesDisjoint(ea.Scope, eb.Scope) {
				continue
			}
			errs = append(errs, fmt.Sprintf(
				"entities.%s and entities.%s both import %q; give them scopes that differ in a value", a, b, ea.Type))
		}
	}
	return errs
}

// scopesDisjoint reports whether no entity can be in both scopes: they set
// one property to different values.
func scopesDisjoint(a, b map[string]string) bool {
	for k, va := range a {
		if vb, ok := b[k]; ok && vb != va {
			return true
		}
	}
	return false
}

func (em *EntityMapping) validate(m *Mapping, meta *metamodel.Metamodel) []string {
	def, ok := meta.GetEntityDef(em.Type)
	if !ok {
		return []string{fmt.Sprintf("type %q is not an entity type", em.Type)}
	}
	var errs []string
	if len(def.Faces) > 0 {
		errs = append(errs, fmt.Sprintf("type %q declares faces, which imports do not support yet", em.Type))
	}
	if p, err := CompilePath(em.Select); err != nil {
		errs = append(errs, "select: "+err.Error())
	} else {
		em.selectPath = p
	}
	errs = append(errs, em.validateIdentity(def)...)
	for _, name := range sortedKeys(em.Properties) {
		v := em.Properties[name]
		pd, known := def.Properties[name]
		switch {
		case !known:
			errs = append(errs, fmt.Sprintf("properties.%s: type %q has no such property", name, em.Type))
		case pd.Computed != "" || pd.Rollup != nil:
			errs = append(errs, fmt.Sprintf("properties.%s: is computed and cannot be imported", name))
		case pd.Type == metamodel.PropertyTypeFile:
			errs = append(errs, fmt.Sprintf("properties.%s: file properties cannot be imported", name))
		}
		if _, scoped := em.Scope[name]; scoped {
			errs = append(errs, fmt.Sprintf("properties.%s: is also in scope", name))
		}
		for _, e := range v.compile() {
			errs = append(errs, fmt.Sprintf("properties.%s: %s", name, e))
		}
	}
	for _, name := range sortedKeys(em.Scope) {
		pd, known := def.Properties[name]
		switch {
		case !known:
			errs = append(errs, fmt.Sprintf("scope.%s: type %q has no such property", name, em.Type))
		case pd.Type == metamodel.PropertyTypeExternalRef || pd.List || pd.Computed != "" || pd.Rollup != nil:
			errs = append(errs, fmt.Sprintf("scope.%s: must be a plain single-valued property", name))
		case em.Scope[name] == "":
			errs = append(errs, fmt.Sprintf("scope.%s: needs a value", name))
		default:
			if _, err := coerce(pd, em.Scope[name]); err != nil {
				errs = append(errs, fmt.Sprintf("scope.%s: %v", name, err))
			}
		}
	}
	if em.Content != nil {
		for _, e := range em.Content.compile() {
			errs = append(errs, "content: "+e)
		}
	}
	errs = append(errs, em.validateRelations(m, meta)...)
	if em.Hook != nil {
		errs = append(errs, em.Hook.validate()...)
	}
	return errs
}

func (em *EntityMapping) validateIdentity(def *metamodel.EntityDef) []string {
	var errs []string
	if em.Key == "" && em.ID == "" {
		return []string{"needs a key property or an id template"}
	}
	if em.Key != "" {
		pd, known := def.Properties[em.Key]
		switch {
		case !known:
			errs = append(errs, fmt.Sprintf("key: type %q has no property %q", em.Type, em.Key))
		case !isKeyProperty(pd):
			errs = append(errs, fmt.Sprintf("key: property %q must be unique: or an external_ref", em.Key))
		case em.Properties[em.Key] == nil:
			errs = append(errs, fmt.Sprintf("key: property %q is not mapped under properties", em.Key))
		}
	}
	if em.ID == "" && def.IsManualID() {
		errs = append(errs, fmt.Sprintf("id: type %q has manual ids, so the mapping needs an id template", em.Type))
	}
	if em.ID != "" && !def.IsManualID() {
		errs = append(errs, fmt.Sprintf("id: type %q generates its ids (id_type: manual is needed for an id template)",
			em.Type))
	}
	if em.ID != "" {
		t, err := compileTemplate(em.ID)
		if err != nil {
			errs = append(errs, "id: "+err.Error())
		}
		em.idParts = t
	}
	return errs
}

func isKeyProperty(pd metamodel.PropertyDef) bool {
	return !pd.List && (pd.Unique || pd.Type == metamodel.PropertyTypeExternalRef)
}

func (em *EntityMapping) validateRelations(m *Mapping, meta *metamodel.Metamodel) []string {
	var errs []string
	names := map[string]bool{}
	for i, rm := range em.Relations {
		at := fmt.Sprintf("relations[%d]", i)
		if rm == nil {
			errs = append(errs, at+": is empty")
			continue
		}
		if rm.Name == "" {
			rm.Name = rm.Type
		}
		if names[rm.Name] {
			errs = append(errs, fmt.Sprintf("%s: name %q is used twice (set name:)", at, rm.Name))
		}
		names[rm.Name] = true
		for _, e := range rm.validate(em, m, meta) {
			errs = append(errs, at+": "+e)
		}
	}
	return append(errs, em.validatePrune()...)
}

// validatePrune refuses a pruning relation mapping next to another mapping
// of the same relation type and direction. The pruning one would remove
// the relations the other adds, and every run would undo the last.
func (em *EntityMapping) validatePrune() []string {
	var errs []string
	for i, a := range em.Relations {
		for _, b := range em.Relations[i+1:] {
			if a == nil || b == nil || a.Type != b.Type || a.out() != b.out() || !a.Prune && !b.Prune {
				continue
			}
			errs = append(errs, fmt.Sprintf(
				"relations %q and %q share type %q and direction; prune cannot be set on either", a.Name, b.Name, a.Type))
		}
	}
	return errs
}

func (rm *RelationMapping) out() bool { return rm.Direction != "in" }

func (rm *RelationMapping) validate(em *EntityMapping, m *Mapping, meta *metamodel.Metamodel) []string {
	var errs []string
	rd, ok := meta.GetRelationDef(rm.Type)
	if !ok {
		return []string{fmt.Sprintf("type %q is not a relation type", rm.Type)}
	}
	targetType := ""
	switch {
	case rm.Target != "" && rm.TargetType != "":
		errs = append(errs, "set target or target_type, not both")
	case rm.Target != "":
		tm := m.Entities[rm.Target]
		if tm == nil {
			errs = append(errs, fmt.Sprintf("target %q is not an entity mapping in this file", rm.Target))
		} else {
			targetType = tm.Type
		}
		if rm.TargetKey != "" {
			errs = append(errs, "target_key goes with target_type; a target mapping matches on its own key")
		}
	case rm.TargetType != "":
		targetType = rm.TargetType
		def, known := meta.GetEntityDef(rm.TargetType)
		switch {
		case !known:
			errs = append(errs, fmt.Sprintf("target_type %q is not an entity type", rm.TargetType))
		case len(def.Faces) > 0:
			errs = append(errs, fmt.Sprintf("target_type %q declares faces, which imports do not support yet",
				rm.TargetType))
		case rm.TargetKey == "":
			errs = append(errs, "target_type needs target_key")
		default:
			if pd, has := def.Properties[rm.TargetKey]; !has || !isKeyProperty(pd) {
				errs = append(errs, fmt.Sprintf("target_key %q must be a unique: or external_ref property of %q",
					rm.TargetKey, rm.TargetType))
			}
		}
	default:
		errs = append(errs, "needs target or target_type")
	}
	from, to := em.Type, targetType
	switch rm.Direction {
	case "", "out":
	case "in":
		from, to = to, from
	default:
		errs = append(errs, fmt.Sprintf("direction %q is not in or out", rm.Direction))
	}
	if targetType != "" && (!allowsType(rd.From, from) || !allowsType(rd.To, to)) {
		errs = append(errs, fmt.Sprintf("relation %q does not connect %s to %s", rm.Type, from, to))
	}
	if rm.Key == nil {
		errs = append(errs, "needs key")
	} else {
		for _, e := range rm.Key.compile() {
			errs = append(errs, "key: "+e)
		}
	}
	return errs
}

func allowsType(types []string, name string) bool {
	return len(types) == 0 || slices.Contains(types, name) || slices.Contains(types, "*")
}

func (h *Hook) validate() []string {
	var errs []string
	clean := path.Clean(h.Script)
	switch {
	case h.Script == "":
		errs = append(errs, "hook: needs a script")
	case !fs.ValidPath(clean) || clean == "." || strings.HasPrefix(clean, "../"):
		errs = append(errs, fmt.Sprintf("hook: script %q must be a project-relative path", h.Script))
	case !strings.HasSuffix(clean, ".lua"):
		errs = append(errs, fmt.Sprintf("hook: script %q must be a .lua file", h.Script))
	}
	if !luaIdent.MatchString(h.function()) {
		errs = append(errs, fmt.Sprintf("hook: function %q is not a Lua name", h.Function))
	}
	return errs
}

var luaIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// compile checks the value's shape and compiles its path or template.
func (v *Value) compile() []string {
	if v == nil {
		return []string{"is empty"}
	}
	set := 0
	for _, has := range []bool{v.Path != "", v.Template != "", v.Const != nil} {
		if has {
			set++
		}
	}
	if set != 1 {
		return []string{"set exactly one of path, template or value"}
	}
	switch {
	case v.Path != "":
		p, err := CompilePath(v.Path)
		if err != nil {
			return []string{err.Error()}
		}
		v.path = p
	case v.Template != "":
		t, err := compileTemplate(v.Template)
		if err != nil {
			return []string{err.Error()}
		}
		v.tmpl = t
	default:
		switch v.Const.(type) {
		case string, int, int64, float64, bool:
		default:
			return []string{"value must be a string, number or boolean"}
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
