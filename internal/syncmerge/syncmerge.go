// Package syncmerge is the pure three-way merge behind Lua rela.sync.merge
// (TKT-SM20FG). A sync connector reads three states of one entity: base (as
// of its last sync, the version tagged sync/<system>), ours (rela now) and
// theirs (the other system now). Merge classifies each synced field:
//
//   - ours equals theirs: unchanged;
//   - base equals ours: only they changed, write theirs into rela;
//   - base equals theirs: only we changed, push ours to them;
//   - otherwise both changed: a conflict, resolved by neither.
//
// Without a base every differing field is a conflict and BaseUnknown is
// set. Equality is per property type (see [Equal]), so a value that differs
// only in form (an integer as 3 or "3", a date with a midnight time) is not
// a change and stops echoing between the systems.
//
// No I/O; the package only reads the metamodel to type the fields.
package syncmerge

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// ContentField is the field name that means the entity body.
const ContentField = "content"

// Kind is how a field compares and canonicalizes.
type Kind int

// Field kinds.
const (
	KindString Kind = iota
	KindEnum
	KindInteger
	KindBoolean
	KindDate
	KindDatetime
	KindExternalRef
	KindContent
)

// Field is one synced field.
type Field struct {
	Name   string
	Kind   Kind
	List   bool
	Values []string // enum values
	Format string   // date/datetime layout for written values
}

// ErrField reports a field the merge refuses: undeclared, unsupported, or
// a value that cannot be coerced.
var ErrField = errors.New("syncmerge")

// emptyMarker is the type of [Empty].
type emptyMarker struct{}

// Empty is a theirs value meaning "they cleared it". An absent theirs key
// means "not reported" and the field is skipped.
var Empty any = emptyMarker{}

// FieldsFor types names against entityType. It refuses an undeclared name,
// a file or computed property, and "content" when the type declares a
// property named content.
func FieldsFor(m *metamodel.Metamodel, entityType string, names []string) ([]Field, error) {
	def, ok := m.GetEntityDef(entityType)
	if !ok {
		return nil, fmt.Errorf("%w: unknown entity type %q", ErrField, entityType)
	}
	props := def.PropertyDefs()
	out := make([]Field, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			return nil, fmt.Errorf("%w: field %q named twice", ErrField, name)
		}
		seen[name] = true
		pd, declared := props[name]
		if name == ContentField {
			if declared {
				return nil, fmt.Errorf("%w: type %q declares a property named content, so the body cannot be synced",
					ErrField, entityType)
			}
			out = append(out, Field{Name: name, Kind: KindContent})
			continue
		}
		if !declared {
			return nil, fmt.Errorf("%w: field %q is not declared on type %q", ErrField, name, entityType)
		}
		f, err := fieldFor(m, name, &pd)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func fieldFor(m *metamodel.Metamodel, name string, pd *metamodel.PropertyDef) (Field, error) {
	f := Field{Name: name, List: pd.List, Format: pd.GetDateFormat()}
	if pd.Computed != "" {
		return f, fmt.Errorf("%w: field %q is computed", ErrField, name)
	}
	switch pd.Type {
	case metamodel.PropertyTypeString, metamodel.PropertyTypeRrule, "":
		f.Kind = KindString
	case metamodel.PropertyTypeInteger:
		f.Kind = KindInteger
	case metamodel.PropertyTypeBoolean:
		f.Kind = KindBoolean
	case metamodel.PropertyTypeDate:
		f.Kind = KindDate
	case metamodel.PropertyTypeDatetime:
		f.Kind = KindDatetime
	case metamodel.PropertyTypeExternalRef:
		f.Kind = KindExternalRef
	case metamodel.PropertyTypeFile:
		return f, fmt.Errorf("%w: field %q is a file property", ErrField, name)
	case metamodel.PropertyTypeEnum:
		f.Kind, f.Values = KindEnum, pd.Values
	default:
		f.Kind = KindString
		if ct, ok := m.Types[pd.Type]; ok && len(ct.Values) > 0 {
			f.Kind, f.Values = KindEnum, ct.Values
		}
	}
	if len(pd.Values) > 0 {
		f.Kind, f.Values = KindEnum, pd.Values
	}
	return f, nil
}

// State is one side: properties, the body, and the names its reader
// withheld.
type State struct {
	Properties map[string]any
	Content    string
	Redacted   []string
}

// Theirs is the other system's state. A property key that is absent was not
// reported; [Empty] clears. Content nil means not reported.
type Theirs struct {
	Properties map[string]any
	Content    *string
}

// Conflict is one field both sides changed.
type Conflict struct {
	Field              string
	Base, Ours, Theirs any
}

// Result is the classification.
type Result struct {
	// BaseUnknown: there was no base, so nothing is written or pushed.
	BaseUnknown bool
	// Write holds canonical theirs values to write into rela; nil clears.
	Write map[string]any
	// WriteContent is the body to write, or nil.
	WriteContent *string
	// Push holds ours values to send to the other system. The body, when
	// pushed, is under [ContentField].
	Push      map[string]any
	Conflicts []Conflict
	Unchanged []string
	// Complete: theirs reported every synced field. A field theirs did not
	// report was not compared, so a local edit to it may still be unpushed.
	Complete bool
	// Retag: the report was [Result.Complete], nothing is to be written or
	// pushed, there are no conflicts, and the base is unknown or differs
	// from ours on a field both sides agree on. Both sides converged, so the
	// caller tags the state it read as the new base.
	Retag bool
}

// Merge classifies fields. base may be nil. A field withheld from ours' or
// base's reader is an error: merging against a hidden value would write or
// push a guess.
func Merge(fields []Field, base *State, ours State, theirs Theirs) (Result, error) {
	res := Result{Write: map[string]any{}, Push: map[string]any{}, BaseUnknown: base == nil, Complete: true}
	baseDiffers := false
	for _, f := range fields {
		if slices.Contains(ours.Redacted, f.Name) || (base != nil && slices.Contains(base.Redacted, f.Name)) {
			return Result{}, fmt.Errorf("%w: field %q is hidden from this reader", ErrField, f.Name)
		}
		o := valueOf(f, ours)
		var b any
		if base != nil {
			b = valueOf(f, *base)
		}
		t, reported, err := theirsValue(f, theirs)
		if err != nil {
			return Result{}, err
		}
		if !reported {
			res.Complete = false
			continue
		}
		switch {
		case Equal(f, o, t):
			res.Unchanged = append(res.Unchanged, f.Name)
			if base == nil || !Equal(f, b, o) {
				baseDiffers = true
			}
		case base == nil:
			res.Conflicts = append(res.Conflicts, Conflict{Field: f.Name, Ours: o, Theirs: t})
		case Equal(f, b, o):
			if f.Kind == KindContent {
				s, _ := t.(string)
				res.WriteContent = &s
			} else {
				res.Write[f.Name] = t
			}
		case Equal(f, b, t):
			res.Push[f.Name] = o
		default:
			res.Conflicts = append(res.Conflicts, Conflict{Field: f.Name, Base: b, Ours: o, Theirs: t})
		}
	}
	res.Retag = res.Complete && len(res.Conflicts) == 0 && len(res.Write) == 0 && res.WriteContent == nil &&
		len(res.Push) == 0 && baseDiffers
	sort.Strings(res.Unchanged)
	return res, nil
}

func valueOf(f Field, s State) any {
	if f.Kind == KindContent {
		return s.Content
	}
	return s.Properties[f.Name]
}

// theirsValue returns the canonical theirs value of f and whether it was
// reported. [Empty] canonicalizes to nil.
func theirsValue(f Field, th Theirs) (value any, reported bool, err error) {
	if f.Kind == KindContent {
		if th.Content == nil {
			return nil, false, nil
		}
		return normalizeContent(*th.Content), true, nil
	}
	v, ok := th.Properties[f.Name]
	if !ok {
		return nil, false, nil
	}
	if v == Empty || metamodel.IsEmptyValue(v) {
		return nil, true, nil
	}
	c, err := Canonical(f, v)
	if err != nil {
		return nil, true, err
	}
	return c, true, nil
}

// Canonical coerces v to f's stored form, or fails when v does not fit.
// nil is nil.
func Canonical(f Field, v any) (any, error) {
	if v == nil || v == Empty {
		return nil, nil //nolint:nilnil // nil is the canonical form of "no value"
	}
	if f.List {
		return canonicalList(f, v)
	}
	switch f.Kind {
	case KindContent:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%w: content must be a string, got %T", ErrField, v)
		}
		return normalizeContent(s), nil
	case KindInteger:
		n, err := metamodel.ParseIntegerValue(v)
		if err != nil {
			return nil, fmt.Errorf("%w: field %q: %w", ErrField, f.Name, err)
		}
		return int64(n), nil
	case KindBoolean:
		switch b := v.(type) {
		case bool:
			return b, nil
		case string:
			if p, err := strconv.ParseBool(b); err == nil {
				return p, nil
			}
		}
		return nil, fmt.Errorf("%w: field %q: %v is not a boolean", ErrField, f.Name, v)
	case KindDate, KindDatetime:
		t, err := asTime(v)
		if err != nil {
			return nil, fmt.Errorf("%w: field %q: %w", ErrField, f.Name, err)
		}
		layout := f.Format
		if layout == "" {
			layout = metamodel.DefaultDateFormat
		}
		if f.Kind == KindDatetime {
			t = t.UTC()
		}
		return t.Format(layout), nil
	case KindEnum:
		s := scalarString(v)
		if !slices.Contains(f.Values, s) {
			return nil, fmt.Errorf("%w: field %q: %q is not one of %v", ErrField, f.Name, s, f.Values)
		}
		return s, nil
	case KindExternalRef:
		r, err := metamodel.ParseExternalRef(v)
		if err != nil {
			return nil, fmt.Errorf("%w: field %q: %w", ErrField, f.Name, err)
		}
		return r.Map(), nil
	default:
		return scalarString(v), nil
	}
}

// canonicalList is [Canonical] for a list field: each item canonicalized.
func canonicalList(f Field, v any) (any, error) {
	items, ok := asList(v)
	if !ok {
		return nil, fmt.Errorf("%w: field %q: want a list, got %T", ErrField, f.Name, v)
	}
	out := make([]any, 0, len(items))
	el := f
	el.List = false
	for _, it := range items {
		c, err := Canonical(el, it)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Equal compares two values of f by meaning: both empty are equal; strings
// exactly; integers numerically; dates by calendar day; datetimes by
// instant; lists as multisets; refs by id; the body after CRLF and trailing
// whitespace normalization. A value that does not fit f compares by its
// string form.
func Equal(f Field, a, b any) bool {
	ea, eb := isEmpty(a), isEmpty(b)
	if ea || eb {
		return ea && eb
	}
	if f.List {
		la, oka := asList(a)
		lb, okb := asList(b)
		if !oka || !okb || len(la) != len(lb) {
			return false
		}
		el := f
		el.List = false
		ka, kb := make([]string, len(la)), make([]string, len(lb))
		for i := range la {
			ka[i] = key(el, la[i])
			kb[i] = key(el, lb[i])
		}
		sort.Strings(ka)
		sort.Strings(kb)
		return slices.Equal(ka, kb)
	}
	return key(f, a) == key(f, b)
}

// key is v's comparison form under f.
func key(f Field, v any) string {
	switch f.Kind {
	case KindContent:
		return "c:" + normalizeContent(scalarString(v))
	case KindDate:
		if t, err := asTime(v); err == nil {
			return "d:" + t.Format("2006-01-02")
		}
	case KindDatetime:
		if t, err := asTime(v); err == nil {
			return "t:" + t.UTC().Format(time.RFC3339Nano)
		}
	case KindExternalRef:
		if id, ok := metamodel.ExternalRefID(v); ok {
			return "r:" + id
		}
	default:
		if c, err := Canonical(f, v); err == nil {
			return fmt.Sprintf("v:%T:%v", c, c)
		}
	}
	return "s:" + scalarString(v)
}

func isEmpty(v any) bool {
	return v == Empty || metamodel.IsEmptyValue(v)
}

func normalizeContent(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n \t")
}

func asList(v any) ([]any, bool) {
	switch l := v.(type) {
	case []any:
		return l, true
	case []string:
		out := make([]any, len(l))
		for i, s := range l {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}

func asTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case string:
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
			if p, err := time.Parse(layout, t); err == nil {
				return p, nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("%v is not a date", v)
}

func scalarString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}
