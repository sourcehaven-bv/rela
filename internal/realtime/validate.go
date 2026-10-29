package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/texthazard"
)

// validProps is a message's properties after validation: every value in
// its stored form, every name declared by the message type.
type validProps struct {
	props map[string]any
	// size is the length of the JSON encoding of props.
	size int
	// refs lists the entity ids the ref properties name, sorted.
	refs []string
}

// validateProps checks in against mt and returns the stored form. It
// refuses an undeclared property, a value of the wrong type, a missing
// required property, an enum value outside its values, a string over its
// max, more than [channels.MaxProps] properties, and an encoding over
// maxBytes (413). A JSON null and an empty list are an absent property.
func validateProps(mt *realtimecfg.MessageType, in map[string]any, maxBytes int) (validProps, error) {
	if len(in) > channels.MaxProps {
		return validProps{}, deny(http.StatusBadRequest, TypeTooManyProps,
			fmt.Sprintf("a message has at most %d properties", channels.MaxProps))
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
			return validProps{}, invalid("message type %s has no property %q", mt.Name, name)
		}
		v, err := normalizeProp(p, raw)
		if err != nil {
			return validProps{}, invalid("property %q: %s", name, err.Error())
		}
		if list, isList := v.([]any); isList && len(list) == 0 {
			continue
		}
		out[name] = v
		if id, ok := v.(string); ok && p.Type == realtimecfg.PropertyRef {
			refs = append(refs, id)
		}
	}
	for _, name := range sortedKeys(mt.Properties) {
		if _, ok := out[name]; !ok && mt.Properties[name].Required {
			return validProps{}, invalid("property %q is required", name)
		}
	}
	raw, err := json.Marshal(out)
	if err != nil { // coverage-ignore: unreachable: normalized values always marshal
		return validProps{}, err
	}
	if len(raw) > maxBytes {
		return validProps{}, deny(http.StatusRequestEntityTooLarge, TypeTooLarge,
			fmt.Sprintf("the message properties are over %d bytes", maxBytes))
	}
	slices.Sort(refs)
	return validProps{props: out, size: len(raw), refs: slices.Compact(refs)}, nil
}

// normalizeProp checks raw against p and returns its stored form: a string
// for string, markdown, enum, ref, date and datetime (a datetime in
// RFC 3339, UTC); an int64 for integer; a bool for boolean; a []any of
// strings for a list.
func normalizeProp(p realtimecfg.Property, raw any) (any, error) {
	if p.List {
		return normalizeList(p, raw)
	}
	switch p.Type {
	case realtimecfg.PropertyInteger:
		return normalizeInt(raw)
	case realtimecfg.PropertyBoolean:
		b, ok := raw.(bool)
		if !ok {
			return nil, errors.New("must be a boolean")
		}
		return b, nil
	case realtimecfg.PropertyString, realtimecfg.PropertyMarkdown, realtimecfg.PropertyDate,
		realtimecfg.PropertyDatetime, realtimecfg.PropertyRef, realtimecfg.PropertyEnum:
	}
	s, ok := raw.(string)
	if !ok {
		return nil, errors.New("must be a string")
	}
	if !utf8.ValidString(s) {
		return nil, errors.New("must be valid UTF-8")
	}
	switch p.Type {
	case realtimecfg.PropertyDate:
		t, err := time.Parse(metamodel.DefaultDateFormat, s)
		if err != nil {
			return nil, errors.New("must be a date like 2006-01-02")
		}
		return t.Format(metamodel.DefaultDateFormat), nil
	case realtimecfg.PropertyDatetime:
		t, err := time.Parse(metamodel.DefaultDatetimeFormat, s)
		if err != nil {
			return nil, errors.New("must be an RFC 3339 date and time")
		}
		return t.UTC().Format(metamodel.DefaultDatetimeFormat), nil
	case realtimecfg.PropertyRef:
		if err := entity.ValidateID(s); err != nil {
			return nil, errors.New("must be an entity id")
		}
		return s, nil
	case realtimecfg.PropertyEnum:
		if !slices.Contains(p.Values, s) {
			return nil, fmt.Errorf("must be one of %v", p.Values)
		}
		return s, nil
	case realtimecfg.PropertyString, realtimecfg.PropertyMarkdown,
		realtimecfg.PropertyInteger, realtimecfg.PropertyBoolean:
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
func normalizeList(p realtimecfg.Property, raw any) (any, error) {
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
func normalizeListItem(p realtimecfg.Property, item any) (string, error) {
	s, ok := item.(string)
	if !ok {
		return "", errors.New("must be a string")
	}
	if !utf8.ValidString(s) {
		return "", errors.New("must be valid UTF-8")
	}
	s = norm.NFC.String(strings.TrimSpace(texthazard.DropStyleSelectors(s)))
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

// whenHolds evaluates mt's `when:` over props in their stored form. A
// program that fails to evaluate does not hold.
func whenHolds(ctx context.Context, mt *realtimecfg.MessageType, props map[string]any, now time.Time) bool {
	if mt.When == nil {
		return true
	}
	fields := make(map[string]predicate.Value, len(mt.Properties))
	for name, p := range mt.Properties {
		fields[name] = predicateValue(p, props[name])
	}
	return mt.Holds(ctx, fields, now)
}

// predicateValue binds a stored property value as the type the `message`
// record declares for it (see realtimecfg's messageEnv). An absent value
// is nil.
func predicateValue(p realtimecfg.Property, v any) predicate.Value {
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
	case realtimecfg.PropertyInteger:
		if n, ok := v.(int64); ok {
			return predicate.NewInt(n)
		}
	case realtimecfg.PropertyBoolean:
		if b, ok := v.(bool); ok {
			return predicate.NewBool(b)
		}
	case realtimecfg.PropertyDate, realtimecfg.PropertyDatetime:
		layout := metamodel.DefaultDateFormat
		if p.Type == realtimecfg.PropertyDatetime {
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

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
