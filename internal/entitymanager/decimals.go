package entitymanager

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// storeDecimalsAsText rewrites each decimal property value that arrived as a
// number (a JSON or Lua number, a YAML float) as its decimal text, so a
// decimal is always stored as a string whatever the client sent
// (TKT-DTUAG3U). A float64 has already lost any digits past about 15
// significant ones; the text keeps what is left exactly. A value that is not
// a valid decimal is left as it is, for validation to report.
func storeDecimalsAsText(meta *metamodel.Metamodel, entityType string, props map[string]any) {
	def, ok := meta.GetEntityDef(entityType)
	if !ok {
		return
	}
	for name, pd := range def.Properties {
		if pd.Type != metamodel.PropertyTypeDecimal {
			continue
		}
		v, ok := props[name]
		if !ok {
			continue
		}
		if list, isList := v.([]any); isList {
			out := make([]any, len(list))
			for i, item := range list {
				out[i] = decimalAsText(item)
			}
			props[name] = out
			continue
		}
		props[name] = decimalAsText(v)
	}
}

func decimalAsText(v any) any {
	if _, isText := v.(string); isText || v == nil {
		return v
	}
	s, err := metamodel.DecimalText(v)
	if err != nil {
		return v
	}
	return s
}
