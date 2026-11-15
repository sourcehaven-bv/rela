package mapimport

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// draft is the entity one record describes, before it is matched with the
// stored rows.
type draft struct {
	em     *EntityMapping
	loc    string
	key    string // the identifying value: the key property's text, or the id
	id     string // the id template's result, "" when there is none
	props  map[string]any
	absent []string // mapped properties with no source value
	// content is nil when the mapping has no content or the source has none.
	content *string
	rels    map[string][]string // relation mapping name → target keys
}

// recordError is a problem with one record. The record is left out of the
// plan; the rest of the import goes on.
type recordError struct {
	mapping, loc, msg string
}

func (e *recordError) Error() string { return e.msg }

func recErr(em *EntityMapping, loc, format string, args ...any) *recordError {
	return &recordError{mapping: em.name, loc: loc, msg: fmt.Sprintf(format, args...)}
}

// derive builds the draft for one record. A nil draft with no error means
// the hook skipped the record.
//
// Nil: hook is accepted; a mapping without a hook has none.
func derive(
	em *EntityMapping, def *metamodel.EntityDef, rec, root *Node, hook *hookRunner,
) (*draft, *recordError) {
	d := &draft{em: em, loc: rec.Location(), props: map[string]any{}, rels: map[string][]string{}}
	for _, name := range sortedKeys(em.Properties) {
		v, present, err := em.Properties[name].property(def.Properties[name], rec, root)
		if err != nil {
			return nil, recErr(em, d.loc, "%s: %v", name, err)
		}
		if present {
			d.props[name] = v
		} else {
			d.absent = append(d.absent, name)
		}
	}
	if em.Content != nil {
		vals, err := em.Content.strings(rec, root)
		if err != nil {
			return nil, recErr(em, d.loc, "content: %v", err)
		}
		if body, ok, err := single(vals, em.Content.Join, "\n\n"); err != nil {
			return nil, recErr(em, d.loc, "content: %v", err)
		} else if ok {
			d.content = &body
		}
	}
	for _, rm := range em.Relations {
		keys, err := rm.Key.strings(rec, root)
		if err != nil {
			return nil, recErr(em, d.loc, "relations.%s: %v", rm.Name, err)
		}
		d.rels[rm.Name] = dedup(keys)
	}
	if em.idParts != nil {
		id, err := em.idParts.render(rec, root, nil)
		if err != nil {
			return nil, recErr(em, d.loc, "id: %v", err)
		}
		d.id = id
	}
	d.key = d.identity(def)
	if hook != nil {
		keep, err := hook.apply(d, def, rec)
		if err != nil {
			return nil, recErr(em, d.loc, "hook: %v", err)
		}
		if !keep {
			return nil, nil
		}
	}
	return d, d.check(def)
}

// identity is the value that matches this draft with a stored entity.
func (d *draft) identity(def *metamodel.EntityDef) string {
	if d.em.Key == "" {
		return d.id
	}
	return keyText(def.Properties[d.em.Key], d.props[d.em.Key])
}

// check validates the finished draft.
func (d *draft) check(def *metamodel.EntityDef) *recordError {
	if d.key == "" {
		if d.em.Key != "" {
			return recErr(d.em, d.loc, "key property %s has no value", d.em.Key)
		}
		return recErr(d.em, d.loc, "id template yields an empty id")
	}
	if d.id != "" {
		if err := entity.ValidateID(d.id); err != nil {
			return recErr(d.em, d.loc, "id %q: %v", d.id, err)
		}
	}
	for name := range d.props {
		pd, known := def.Properties[name]
		if !known || pd.Computed != "" || pd.Rollup != nil || pd.Type == metamodel.PropertyTypeFile {
			return recErr(d.em, d.loc, "property %s cannot be imported into %s", name, d.em.Type)
		}
	}
	for _, rm := range d.em.Relations {
		if len(d.rels[rm.Name]) == 0 && !rm.Optional {
			return recErr(d.em, d.loc, "relations.%s: no target key (set optional: true if that is expected)",
				rm.Name)
		}
	}
	return nil
}

// keyText is the text a key value is compared by: an external ref's id, or
// the value itself.
func keyText(pd metamodel.PropertyDef, v any) string {
	if pd.Type == metamodel.PropertyTypeExternalRef {
		id, _ := metamodel.ExternalRefID(v)
		return id
	}
	return normalize(v)
}

// strings evaluates the value for one record: every non-empty match.
func (v *Value) strings(rec, root *Node) ([]string, error) {
	switch {
	case v.path != nil:
		return texts(v.path, rec, root)
	case v.tmpl != nil:
		s, err := v.tmpl.render(rec, root, v.Join)
		if err != nil || s == "" {
			return nil, err
		}
		return []string{s}, nil
	default:
		s := fmt.Sprint(v.Const)
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	}
}

// property derives the value of one property. present is false when the
// source has no value for it.
func (v *Value) property(pd metamodel.PropertyDef, rec, root *Node) (value any, present bool, err error) {
	vals, err := v.strings(rec, root)
	if err != nil {
		return nil, false, err
	}
	if pd.List {
		if len(vals) == 0 {
			return nil, false, nil
		}
		out := make([]any, 0, len(vals))
		for _, s := range vals {
			c, cerr := coerce(pd, s)
			if cerr != nil {
				return nil, false, cerr
			}
			out = append(out, c)
		}
		return out, true, nil
	}
	s, ok, err := single(vals, v.Join, "")
	if err != nil || !ok {
		return nil, false, err
	}
	c, err := coerce(pd, s)
	return c, err == nil, err
}

// single reduces vals to one string: the only one, or all joined with join,
// or with fallback when join is unset. An empty fallback makes several
// values an error.
func single(vals []string, join *string, fallback string) (joined string, ok bool, err error) {
	switch {
	case len(vals) == 0:
		return "", false, nil
	case len(vals) == 1:
		return vals[0], true, nil
	case join != nil:
		return strings.Join(vals, *join), true, nil
	case fallback != "":
		return strings.Join(vals, fallback), true, nil
	}
	return "", false, fmt.Errorf("matched %d values; set join or narrow the path", len(vals))
}

// coerce converts source text to the stored form of pd's type. Types the
// manager validates as strings (dates, enums, decimals) stay strings.
func coerce(pd metamodel.PropertyDef, s string) (any, error) {
	switch pd.Type {
	case metamodel.PropertyTypeInteger:
		t := strings.TrimSpace(s)
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return n, nil
		}
		// Exporters write whole numbers as 3.0 or 1e3 too.
		f, err := strconv.ParseFloat(t, 64)
		if err != nil || f != math.Trunc(f) || math.Abs(f) > 1<<53 {
			return nil, fmt.Errorf("%q is not an integer", s)
		}
		return int64(f), nil
	case metamodel.PropertyTypeBoolean:
		b, err := strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("%q is not a boolean", s)
		}
		return b, nil
	case metamodel.PropertyTypeExternalRef:
		ref, err := metamodel.ParseExternalRef(map[string]any{"id": s})
		if err != nil {
			return nil, err
		}
		return ref.Map(), nil
	}
	return s, nil
}

func dedup(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
