package affordances

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// FieldWriteRule names why a property write was refused. The values are a
// wire contract: dataentry serves them as the 403 `rule_id` prefix.
type FieldWriteRule string

const (
	RuleFieldHidden       FieldWriteRule = "field-affordance:hidden"
	RuleFieldReadOnly     FieldWriteRule = "field-affordance:read-only"
	RuleFieldEnumFiltered FieldWriteRule = "field-affordance:enum-filtered"
)

// FieldWriteError is a refused property write. Path is the property name,
// or "field=option" for a filtered enum option.
type FieldWriteError struct {
	Rule   FieldWriteRule
	Path   string
	Reason string
	// Attribution names the role or grant that denied, for audit only.
	Attribution string
}

// RuleID returns the wire identifier, "<rule>:<path>".
func (d *FieldWriteError) RuleID() string {
	if d.Path == "" {
		return string(d.Rule)
	}
	return string(d.Rule) + ":" + d.Path
}

func (d *FieldWriteError) Error() string {
	return d.RuleID() + ": " + d.Reason
}

// AuditSummary is the denied-write audit summary, in the format dataentry
// records for the same refusal. It carries the attribution the wire omits.
func (d *FieldWriteError) AuditSummary() string {
	s := fmt.Sprintf("denied: %s (rule_kind=affordance rule_id=%s)", d.Reason, d.RuleID())
	if d.Attribution != "" {
		s += " attribution=" + d.Attribution
	}
	return s
}

// CheckFieldWrite returns the first denial the proposed writes trigger, or
// nil. set maps each property being written to its new value; unset names
// the properties being removed. Four classes are refused:
//
//  1. A field neither declared nor known to the resolver, refused exactly
//     like a hidden one, so a caller cannot probe which names exist (F8).
//  2. A hidden field.
//  3. A read-only field. A write of the unchanged value is refused too.
//  4. A filtered enum option, scalar or list. Only a set carries a value.
//
// Set keys are checked before unset keys, each in name order, so the
// denial reported for a multi-field write does not vary between calls.
func CheckFieldWrite(v FieldVerdicts, declared map[string]bool, set map[string]any, unset []string) *FieldWriteError {
	check := func(key string, value any, isSet bool) *FieldWriteError {
		if !declared[key] && !knownToResolver(v, key) {
			return &FieldWriteError{Rule: RuleFieldHidden, Path: key, Reason: fmt.Sprintf("field %q is not visible", key)}
		}
		if visible, ok := v.Visible[key]; ok && !visible {
			return &FieldWriteError{
				Rule: RuleFieldHidden, Path: key, Reason: fmt.Sprintf("field %q is not visible", key),
				Attribution: v.Attribution[key],
			}
		}
		if writable, ok := v.Writable[key]; ok && !writable {
			return &FieldWriteError{
				Rule: RuleFieldReadOnly, Path: key, Reason: fmt.Sprintf("field %q is not writable", key),
				Attribution: v.Attribution[key],
			}
		}
		if isSet && value != nil {
			if opts, ok := v.Options[key]; ok {
				if d := checkEnumOption(key, value, opts); d != nil {
					d.Attribution = v.Attribution[d.Path]
					return d
				}
			}
		}
		return nil
	}

	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if d := check(k, set[k], true); d != nil {
			return d
		}
	}
	for _, k := range unset {
		if d := check(k, nil, false); d != nil {
			return d
		}
	}
	return nil
}

// checkEnumOption refuses an enum value outside the allow-set. A list value
// is refused on its first disallowed element. Values of other types pass:
// type validation rejects those elsewhere.
func checkEnumOption(key string, value any, opts map[string]bool) *FieldWriteError {
	deny := func(option string) *FieldWriteError {
		return &FieldWriteError{
			Rule:   RuleFieldEnumFiltered,
			Path:   key + "=" + option,
			Reason: fmt.Sprintf("option %q is not allowed for field %q", option, key),
		}
	}
	switch v := value.(type) {
	case string:
		if allowed, ok := opts[v]; ok && !allowed {
			return deny(v)
		}
	case []any:
		for _, elem := range v {
			str, ok := elem.(string)
			if !ok {
				continue
			}
			if allowed, ok := opts[str]; ok && !allowed {
				return deny(str)
			}
		}
	case []string:
		for _, str := range v {
			if allowed, ok := opts[str]; ok && !allowed {
				return deny(str)
			}
		}
	}
	return nil
}

// knownToResolver reports whether the verdicts mention name at all, which
// admits a field the metamodel does not declare but the policy names.
func knownToResolver(v FieldVerdicts, name string) bool {
	if _, ok := v.Writable[name]; ok {
		return true
	}
	if _, ok := v.Visible[name]; ok {
		return true
	}
	_, ok := v.Options[name]
	return ok
}

// DeclaredProperties returns the property names the metamodel declares for
// entityType. An unknown type declares nothing, so every write to it is
// refused as hidden.
func DeclaredProperties(meta *metamodel.Metamodel, entityType string) map[string]bool {
	out := make(map[string]bool)
	if meta == nil {
		return out
	}
	def, ok := meta.Entities[entityType]
	if !ok {
		return out
	}
	for name := range def.Properties {
		out[name] = true
	}
	return out
}

// WriteGate refuses property writes the caller's field grants forbid. It
// satisfies entitymanager.FieldWriteGate; the principal comes from ctx.
type WriteGate struct {
	resolver *PolicyResolver
}

// NewWriteGate returns a gate over r, which must be fully built, machines
// included, because the gate shares it with concurrent callers.
//
// Nil: rejected. A deployment without field grants wires
// entitymanager.AllowAllFieldGate instead.
func NewWriteGate(r *PolicyResolver) (*WriteGate, error) {
	if r == nil {
		return nil, errors.New("affordances: NewWriteGate: resolver is required")
	}
	return &WriteGate{resolver: r}, nil
}

// CheckFieldWrite returns a *[FieldWriteError] for the first refused write,
// or nil.
func (g *WriteGate) CheckFieldWrite(ctx context.Context, e *entity.Entity, set map[string]any, unset []string) error {
	if e == nil {
		return errors.New("affordances: field write check without an entity")
	}
	if len(set) == 0 && len(unset) == 0 {
		return nil // a content-only write names no property
	}
	d := CheckFieldWrite(g.resolver.FieldVerdicts(ctx, e), DeclaredProperties(g.resolver.meta, e.Type), set, unset)
	if d == nil {
		return nil // not d: a nil *FieldWriteError is a non-nil error
	}
	return d
}
