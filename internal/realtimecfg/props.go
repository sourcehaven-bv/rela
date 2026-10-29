package realtimecfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
	"github.com/Sourcehaven-BV/rela/internal/texthazard"
)

// The prop rules of a post live here, beside the message type, so the
// channel service and the load check of a button's preset run the same
// code (TKT-S9TLJJ).

var (
	// ErrTooManyProps is the refusal of more than
	// [realtimelimits.MaxMessageProperties] properties.
	ErrTooManyProps = errors.New("too many properties")
	// ErrTooLarge is the refusal of properties whose encoding is over the
	// byte limit.
	ErrTooLarge = errors.New("properties too large")
)

// NormalizedProps is a message's properties after [MessageType.NormalizeProps]:
// every value in its stored form, every name declared by the message type.
type NormalizedProps struct {
	Props map[string]any
	// Size is the length of the JSON encoding of Props.
	Size int
	// Refs lists the entity ids the ref properties name, sorted.
	Refs []string
}

// NormalizeProps checks in against mt and returns the stored form. It
// refuses an undeclared property, a value of the wrong type, a missing
// required property, an enum value outside its values, a string over its
// max, more than [realtimelimits.MaxMessageProperties] properties
// ([ErrTooManyProps]), and an encoding over maxBytes ([ErrTooLarge]). A
// nil value and an empty list are an absent property.
func (mt *MessageType) NormalizeProps(in map[string]any, maxBytes int) (NormalizedProps, error) {
	if len(in) > realtimelimits.MaxMessageProperties {
		return NormalizedProps{}, fmt.Errorf("%w: a message has at most %d properties",
			ErrTooManyProps, realtimelimits.MaxMessageProperties)
	}
	out := make(map[string]any, len(in))
	var refs []string
	for _, name := range sortedKeys(in) {
		raw := in[name]
		if raw == nil {
			continue
		}
		p, ok := mt.Properties[name]
		if !ok {
			return NormalizedProps{}, fmt.Errorf("message type %s has no property %q", mt.Name, name)
		}
		v, err := p.Normalize(raw)
		if err != nil {
			return NormalizedProps{}, fmt.Errorf("property %q: %s", name, err.Error())
		}
		if list, isList := v.([]any); isList && len(list) == 0 {
			continue
		}
		out[name] = v
		if id, ok := v.(string); ok && p.Type == PropertyRef {
			refs = append(refs, id)
		}
	}
	for _, name := range sortedKeys(mt.Properties) {
		if _, ok := out[name]; !ok && mt.Properties[name].Required {
			return NormalizedProps{}, fmt.Errorf("property %q is required", name)
		}
	}
	raw, err := json.Marshal(out)
	if err != nil { // coverage-ignore: unreachable: normalized values always marshal
		return NormalizedProps{}, err
	}
	if len(raw) > maxBytes {
		return NormalizedProps{}, fmt.Errorf("%w: the message properties are over %d bytes", ErrTooLarge, maxBytes)
	}
	slices.Sort(refs)
	return NormalizedProps{Props: out, Size: len(raw), Refs: slices.Compact(refs)}, nil
}

// Normalize checks raw against p and returns its stored form: a string
// for string, markdown, enum, ref, date and datetime (a datetime in
// RFC 3339, UTC); an int64 for integer; a bool for boolean; a []any of
// strings for a list.
func (p Property) Normalize(raw any) (any, error) {
	if p.List {
		return normalizeList(p, raw)
	}
	switch p.Type {
	case PropertyInteger:
		return normalizeInt(raw)
	case PropertyBoolean:
		b, ok := raw.(bool)
		if !ok {
			return nil, errors.New("must be a boolean")
		}
		return b, nil
	case PropertyString, PropertyMarkdown, PropertyDate, PropertyDatetime, PropertyRef, PropertyEnum:
	}
	s, ok := raw.(string)
	if !ok {
		return nil, errors.New("must be a string")
	}
	if !utf8.ValidString(s) {
		return nil, errors.New("must be valid UTF-8")
	}
	switch p.Type {
	case PropertyDate:
		t, err := time.Parse(metamodel.DefaultDateFormat, s)
		if err != nil {
			return nil, errors.New("must be a date like 2006-01-02")
		}
		return t.Format(metamodel.DefaultDateFormat), nil
	case PropertyDatetime:
		t, err := time.Parse(metamodel.DefaultDatetimeFormat, s)
		if err != nil {
			return nil, errors.New("must be an RFC 3339 date and time")
		}
		return t.UTC().Format(metamodel.DefaultDatetimeFormat), nil
	case PropertyRef:
		if err := entity.ValidateID(s); err != nil {
			return nil, errors.New("must be an entity id")
		}
		return s, nil
	case PropertyEnum:
		if !slices.Contains(p.Values, s) {
			return nil, fmt.Errorf("must be one of %v", p.Values)
		}
		return s, nil
	case PropertyString, PropertyMarkdown, PropertyInteger, PropertyBoolean:
	}
	if p.Max > 0 && utf8.RuneCountInString(s) > p.Max {
		return nil, fmt.Errorf("must be at most %d characters", p.Max)
	}
	return s, nil
}

// normalizeList checks a list value against p and returns its stored form.
// Each item loses its emoji style selectors and is trimmed and
// NFC-normalized, unlike a scalar string, so two items that look the same
// compare equal. An item must then be non-empty,
// at most p.Max code points and free of display hazards, and differ from
// every other item. The list holds at most p.MaxItems items.
func normalizeList(p Property, raw any) (any, error) {
	var items []any
	switch v := raw.(type) {
	case []any:
		items = v
	case []string:
		items = make([]any, len(v))
		for i, s := range v {
			items[i] = s
		}
	default:
		return nil, errors.New("must be a list of strings")
	}
	if len(items) > p.MaxItems {
		return nil, fmt.Errorf("must have at most %d items", p.MaxItems)
	}
	out := make([]any, 0, len(items))
	seen := make(map[string]bool, len(items))
	for i, item := range items {
		s, err := normalizeListItem(p, item)
		if err != nil {
			return nil, fmt.Errorf("item %d %s", i+1, err.Error())
		}
		if seen[s] {
			return nil, fmt.Errorf("item %d repeats an earlier item", i+1)
		}
		seen[s] = true
		out = append(out, s)
	}
	return out, nil
}

// normalizeListItem checks one item of a list and returns its stored
// form. Its errors read after "item N".
func normalizeListItem(p Property, item any) (string, error) {
	s, ok := item.(string)
	if !ok {
		return "", errors.New("must be a string")
	}
	if !utf8.ValidString(s) {
		return "", errors.New("must be valid UTF-8")
	}
	s = foldDisplay(s)
	switch {
	case s == "":
		return "", errors.New("is empty")
	case utf8.RuneCountInString(s) > p.Max:
		return "", fmt.Errorf("must be at most %d characters", p.Max)
	}
	if r, found := texthazard.Find(s); found {
		return "", fmt.Errorf("holds %U, which displays as something other than what it is", r)
	}
	return s, nil
}

// foldDisplay drops emoji style selectors, trims and NFC-normalizes s, so
// two texts that look the same compare equal.
func foldDisplay(s string) string {
	return norm.NFC.String(strings.TrimSpace(texthazard.DropStyleSelectors(s)))
}

// normalizeInt accepts a whole number from JSON (float64 or json.Number)
// or Go (int, int64).
func normalizeInt(raw any) (any, error) {
	switch v := raw.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		if v == math.Trunc(v) && math.Abs(v) <= 1<<53 {
			return int64(v), nil
		}
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n, nil
		}
	}
	return nil, errors.New("must be a whole number")
}

// PredicateValue binds a stored property value as the type the `message`
// record declares for it (see [messageRecordType]). An absent value is
// nil.
func (p Property) PredicateValue(v any) predicate.Value {
	if v == nil {
		return predicate.NewNil()
	}
	if p.List {
		items, ok := v.([]any)
		if !ok {
			return predicate.NewNil()
		}
		elems := make([]predicate.Value, 0, len(items))
		for _, item := range items {
			if s, isStr := item.(string); isStr {
				elems = append(elems, predicate.NewString(s))
			}
		}
		return predicate.NewList(elems)
	}
	switch p.Type {
	case PropertyInteger:
		if n, ok := v.(int64); ok {
			return predicate.NewInt(n)
		}
	case PropertyBoolean:
		if b, ok := v.(bool); ok {
			return predicate.NewBool(b)
		}
	case PropertyDate, PropertyDatetime:
		layout := metamodel.DefaultDateFormat
		if p.Type == PropertyDatetime {
			layout = metamodel.DefaultDatetimeFormat
		}
		if s, ok := v.(string); ok {
			if t, err := time.Parse(layout, s); err == nil {
				return predicate.NewDate(t)
			}
		}
	default:
		if s, ok := v.(string); ok {
			return predicate.NewString(s)
		}
	}
	return predicate.NewNil()
}

// PredicateFields binds props, in their stored form, as the fields of mt's
// `message` record. An absent property is nil.
func (mt *MessageType) PredicateFields(props map[string]any) map[string]predicate.Value {
	fields := make(map[string]predicate.Value, len(mt.Properties))
	for name, p := range mt.Properties {
		fields[name] = p.PredicateValue(props[name])
	}
	return fields
}
