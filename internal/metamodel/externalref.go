package metamodel

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

// External-ref values (TKT-SM20FG). A property of type `external_ref` links
// an entity to its counterpart in another system. The system lives in the
// schema (`system:`), the value is a map with an `id` and an optional `url`.
// The pair (system, id) is unique per face across every type declaring the
// system; the entitymanager enforces that.

// ExternalRefIDMaxBytes is the longest an external id may be.
const ExternalRefIDMaxBytes = 256

// ExternalRefValue is a parsed external-ref value.
type ExternalRefValue struct {
	ID  string
	URL string
}

// Map returns the stored form of v: {id, url}, with url omitted when empty.
func (v ExternalRefValue) Map() map[string]any {
	m := map[string]any{"id": v.ID}
	if v.URL != "" {
		m["url"] = v.URL
	}
	return m
}

// ErrInvalidExternalRef is the class of every [ParseExternalRef] failure.
var ErrInvalidExternalRef = errors.New("invalid external ref")

// IsEmptyValue reports whether a property value means "no value": nil, the
// empty string or an empty list. It is the one definition of emptiness for
// stored values, and agrees with propmatch.IsEmpty and the store backends'
// SQL emptiness test. A map is never empty: `{}` is a value, and as an
// external ref it has no id, so validation refuses it and it never stores.
func IsEmptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []string:
		return len(t) == 0
	case []any:
		return len(t) == 0
	}
	return false
}

// ParseExternalRef validates v as an external-ref value. The value must be
// a map holding `id` and optionally `url`, and no other key. The id is a
// non-empty string of at most [ExternalRefIDMaxBytes] bytes of printable
// characters (no control or format characters); a number is refused,
// because a large numeric id loses precision on its way through JSON and
// Lua. The url, when present, is an
// http or https URL with a host.
func ParseExternalRef(v any) (ExternalRefValue, error) {
	m, ok := stringKeyedMap(v)
	if !ok {
		return ExternalRefValue{}, fmt.Errorf("%w: must be an object with an \"id\"", ErrInvalidExternalRef)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		if k != "id" && k != "url" {
			keys = append(keys, k)
		}
	}
	if len(keys) > 0 {
		sort.Strings(keys)
		return ExternalRefValue{}, fmt.Errorf("%w: unknown key %q (allowed: id, url)", ErrInvalidExternalRef, keys[0])
	}
	var out ExternalRefValue
	switch id := m["id"].(type) {
	case string:
		if err := validateExternalID(id); err != nil {
			return ExternalRefValue{}, err
		}
		out.ID = id
	case nil:
		return ExternalRefValue{}, fmt.Errorf("%w: \"id\" is required", ErrInvalidExternalRef)
	case int, int64, float64, uint64, int32:
		return ExternalRefValue{}, fmt.Errorf("%w: \"id\" must be a string; quote numeric ids", ErrInvalidExternalRef)
	default:
		return ExternalRefValue{}, fmt.Errorf("%w: \"id\" must be a string", ErrInvalidExternalRef)
	}
	switch u := m["url"].(type) {
	case nil:
	case string:
		if u != "" {
			if err := validateExternalURL(u); err != nil {
				return ExternalRefValue{}, err
			}
		}
		out.URL = u
	default:
		return ExternalRefValue{}, fmt.Errorf("%w: \"url\" must be a string", ErrInvalidExternalRef)
	}
	return out, nil
}

// ExternalRefID returns the id of a stored external-ref value, or false
// when the value is empty or malformed.
func ExternalRefID(v any) (string, bool) {
	if IsEmptyValue(v) {
		return "", false
	}
	ref, err := ParseExternalRef(v)
	if err != nil {
		return "", false
	}
	return ref.ID, true
}

// FormatExternalRef renders a stored external-ref value as text: its id.
// Exports and list cells use it. A malformed value renders as "".
func FormatExternalRef(v any) string {
	id, _ := ExternalRefID(v)
	return id
}

func stringKeyedMap(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		return t, true
	case map[string]string:
		m := make(map[string]any, len(t))
		for k, s := range t {
			m[k] = s
		}
		return m, true
	}
	return nil, false
}

func validateExternalID(id string) error {
	if id == "" {
		return fmt.Errorf("%w: \"id\" must not be empty", ErrInvalidExternalRef)
	}
	if len(id) > ExternalRefIDMaxBytes {
		return fmt.Errorf("%w: \"id\" is longer than %d bytes", ErrInvalidExternalRef, ExternalRefIDMaxBytes)
	}
	// Only printable characters: no controls, no format characters (U+202E
	// right-to-left override, U+200B zero-width space), which would let two
	// ids that render alike differ, or one render as another.
	for _, r := range id {
		if r == unicode.ReplacementChar || unicode.Is(unicode.Cf, r) || !unicode.IsPrint(r) {
			return fmt.Errorf("%w: \"id\" contains a control, format or invalid character", ErrInvalidExternalRef)
		}
	}
	return nil
}

func validateExternalURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: \"url\" is not a URL", ErrInvalidExternalRef)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: \"url\" must use http or https", ErrInvalidExternalRef)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: \"url\" must have a host", ErrInvalidExternalRef)
	}
	return nil
}

// ExternalRefProp names one property declaring an external-ref system.
type ExternalRefProp struct {
	Type     string
	Property string
	// Sync reports `sync: true` on the property.
	Sync bool
}

// ExternalRefProps returns every (type, property) declaring system, sorted
// by type. A type declares a system at most once (enforced at load).
func ExternalRefProps(m *Metamodel, system string) []ExternalRefProp {
	if m == nil {
		return nil
	}
	var out []ExternalRefProp
	for typeName, def := range m.Entities {
		for name, pd := range def.Properties {
			if pd.Type == PropertyTypeExternalRef && pd.System == system {
				out = append(out, ExternalRefProp{Type: typeName, Property: name, Sync: pd.Sync})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// ExternalRefSystem returns the system of entityType's property prop, when
// that property is an external ref.
func ExternalRefSystem(m *Metamodel, entityType, prop string) (string, bool) {
	if m == nil {
		return "", false
	}
	def, ok := m.Entities[entityType]
	if !ok {
		return "", false
	}
	pd, ok := def.Properties[prop]
	if !ok || pd.Type != PropertyTypeExternalRef {
		return "", false
	}
	return pd.System, true
}

// ExternalRefPropertyNames returns entityType's external-ref properties,
// sorted.
func ExternalRefPropertyNames(m *Metamodel, entityType string) []string {
	if m == nil {
		return nil
	}
	def, ok := m.Entities[entityType]
	if !ok {
		return nil
	}
	var out []string
	for name, pd := range def.Properties {
		if pd.Type == PropertyTypeExternalRef {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// SyncRefProps returns "type.property" for every external ref declared
// `sync: true`, sorted. A host that serves sync needs version history.
func SyncRefProps(m *Metamodel) []string {
	if m == nil {
		return nil
	}
	var out []string
	for typeName, def := range m.Entities {
		for name, pd := range def.Properties {
			if pd.Type == PropertyTypeExternalRef && pd.Sync {
				out = append(out, typeName+"."+name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// validateExternalRefOptions checks one property's external-ref options.
// entityProp is false for relation properties, which may not be refs.
func validateExternalRefOptions(schemaName, propName string, pd PropertyDef, entityProp bool) []string {
	if pd.Type != PropertyTypeExternalRef {
		var errs []string
		if pd.System != "" {
			errs = append(errs, fmt.Sprintf("%s: property %q sets \"system\" but is type %q; "+
				"only applies to type \"external_ref\"", schemaName, propName, pd.Type))
		}
		if pd.Sync {
			errs = append(errs, fmt.Sprintf("%s: property %q sets \"sync\" but is type %q; "+
				"only applies to type \"external_ref\"", schemaName, propName, pd.Type))
		}
		return errs
	}
	where := fmt.Sprintf("%s: external_ref property %q", schemaName, propName)
	if !entityProp {
		return []string{where + ": external refs are not supported on relation properties"}
	}
	var errs []string
	if pd.System == "" {
		errs = append(errs, where+": \"system\" is required")
	} else if !validSystemName(pd.System) {
		errs = append(errs, fmt.Sprintf("%s: system %q must match [a-z0-9][a-z0-9._-]{0,62} "+
			"(it names the version tag sync/<system>)", where, pd.System))
	}
	for _, opt := range []struct {
		name string
		set  bool
	}{
		{"list", pd.List},
		{"unique", pd.Unique},
		{"default", pd.Default != ""},
		{"computed", pd.Computed != ""},
		{"values", len(pd.Values) > 0},
		{"format", pd.Format != ""},
		{"required", pd.Required},
	} {
		if opt.set {
			errs = append(errs, fmt.Sprintf("%s: %q cannot be combined with an external ref", where, opt.name))
		}
	}
	return errs
}

// validSystemName reports whether s is a valid version tag segment, the
// grammar of store.ParseVersionTagName's segments. metamodel cannot import
// store; TestValidSystemNameMatchesTagGrammar pins the two together.
func validSystemName(s string) bool {
	const maxSegment = 63
	if s == "" || len(s) > maxSegment {
		return false
	}
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case i > 0 && (c == '.' || c == '_' || c == '-'):
		default:
			return false
		}
	}
	return true
}

// validateExternalRefs checks the cross-property external-ref rules: one
// property per system per type, `sync: true` only on a type without
// faces, and no automation that triggers on a ref or writes one. The
// display_property rule lives in validateDisplayPropertyRef.
func validateExternalRefs(m *Metamodel) []string {
	var errs []string
	for _, typeName := range sortedKeys(m.Entities) {
		def := m.Entities[typeName]
		bySystem := map[string][]string{}
		for _, name := range sortedKeys(def.Properties) {
			pd := def.Properties[name]
			if pd.Type != PropertyTypeExternalRef {
				continue
			}
			if pd.System != "" {
				bySystem[pd.System] = append(bySystem[pd.System], name)
			}
			if pd.Sync && len(def.Faces) > 0 {
				errs = append(errs, fmt.Sprintf("entity %q: external_ref property %q: \"sync: true\" is not "+
					"supported on a type with faces", typeName, name))
			}
		}
		for _, system := range sortedKeys(bySystem) {
			if names := bySystem[system]; len(names) > 1 {
				errs = append(errs, fmt.Sprintf("entity %q: properties %s all declare system %q; "+
					"a type may declare each system once", typeName, strings.Join(names, ", "), system))
			}
		}
	}
	for _, auto := range m.Automations {
		errs = append(errs, automationExternalRefErrors(m, auto)...)
	}
	return errs
}

// automationExternalRefErrors refuses an automation that triggers on an
// external ref or writes one. A `set:` or a `create_entity` property is a
// system write that no field gate sees, and its value is an interpolated
// string, never an {id, url} object: a ref is written only by a script
// granted ref writes.
func automationExternalRefErrors(m *Metamodel, auto AutomationDef) []string {
	types := []string(auto.On.Entity)
	if len(types) == 0 {
		types = sortedKeys(m.Entities)
	}
	var errs []string
	for _, t := range types {
		if auto.On.Property != "" {
			if _, ok := ExternalRefSystem(m, t, auto.On.Property); ok {
				errs = append(errs, fmt.Sprintf("automation %q: `on.property` %q is an external ref on %q; "+
					"a ref changes only through sync, so it cannot trigger an automation", auto.Name, auto.On.Property, t))
			}
		}
		for _, act := range auto.Do {
			if act.Set == "" {
				continue
			}
			if _, ok := ExternalRefSystem(m, t, act.Set); ok {
				errs = append(errs, fmt.Sprintf("automation %q: `set` %q is an external ref on %q; "+
					"only a script may write a ref", auto.Name, act.Set, t))
			}
		}
	}
	for _, act := range auto.Do {
		if act.CreateEntity == nil {
			continue
		}
		for _, name := range sortedKeys(act.CreateEntity.Properties) {
			if _, ok := ExternalRefSystem(m, act.CreateEntity.Type, name); ok {
				errs = append(errs, fmt.Sprintf("automation %q: `create_entity` property %q is an external ref on %q; "+
					"only a script may write a ref", auto.Name, name, act.CreateEntity.Type))
			}
		}
	}
	return errs
}
