package records

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// normalizeFields checks in against the fields of def and returns the
// stored form: unknown fields and missing required ones are refused, and
// each value is converted to string, int64 or bool.
func normalizeFields(meta *metamodel.Metamodel, def metamodel.RecordTypeDef,
	in map[string]any) (map[string]any, error) {
	var problems []string
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]any, len(in))
	bad := map[string]bool{}
	for _, k := range keys {
		p, ok := def.Fields[k]
		if !ok {
			problems = append(problems, k+": unknown field")
			continue
		}
		v := in[k]
		if v == nil {
			continue
		}
		nv, err := normalizeValue(meta, k, p, v)
		if err != nil {
			problems = append(problems, k+": "+err.Error())
			bad[k] = true
			continue
		}
		out[k] = nv
	}
	for _, k := range def.FieldNames() {
		if _, has := out[k]; !has && !bad[k] && def.Fields[k].Required {
			problems = append(problems, k+": required")
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, strings.Join(problems, "; "))
	}
	return out, nil
}

func normalizeValue(meta *metamodel.Metamodel, name string, p metamodel.PropertyDef, v any) (any, error) {
	if n, ok := v.(json.Number); ok {
		v = n.String()
	}
	switch p.Type {
	case metamodel.PropertyTypeInteger:
		i, err := toInt64(v)
		if err != nil {
			return nil, errors.New("must be an integer")
		}
		if err := metamodel.NewValidator(meta).ValidatePropertyValue(name, &p, i); err != nil {
			return nil, err
		}
		return i, nil
	case metamodel.PropertyTypeDecimal:
		if f, isFloat := v.(float64); isFloat {
			v = strconv.FormatFloat(f, 'f', -1, 64)
		}
		if err := metamodel.NewValidator(meta).ValidatePropertyValue(name, &p, v); err != nil {
			return nil, err
		}
		return metamodel.DecimalText(v)
	case metamodel.PropertyTypeBoolean:
		switch b := v.(type) {
		case bool:
			return b, nil
		case string:
			if b == "true" || b == "false" {
				return b == "true", nil
			}
		}
		return nil, errors.New("must be true or false")
	case metamodel.PropertyTypeDatetime:
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("must be a datetime string")
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, errors.New("must be an RFC 3339 datetime")
		}
		return t.UTC().Format(time.RFC3339Nano), nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, errors.New("must be a string")
	}
	if err := metamodel.NewValidator(meta).ValidatePropertyValue(name, &p, s); err != nil {
		return nil, err
	}
	return s, nil
}

func toInt64(v any) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int64:
		return n, nil
	case float64:
		if n != math.Trunc(n) || math.Abs(n) > 1<<53 {
			return 0, strconv.ErrSyntax
		}
		return int64(n), nil
	case string:
		return strconv.ParseInt(n, 10, 64)
	}
	return 0, strconv.ErrSyntax
}
