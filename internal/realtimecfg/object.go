package realtimecfg

import (
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// objectChecker compiles an object list property (`type: object`). It
// keeps that check off the validator, which is at its method budget.
type objectChecker struct{ *validator }

// object compiles p, whose type is object, from raw: a list of objects
// whose fields are raw.Properties. A field takes the keys of any property,
// but only the types fieldType admits, and it may itself be a list of
// strings. The list keys max_items; max, to and notify belong to fields.
func (v objectChecker) object(key string, p Property, raw *rawProperty, limits Limits) Property {
	p.Type, p.List = PropertyObject, true
	if !raw.List {
		v.add(key+".list", "an object property is a list of objects; add list: true")
	}
	if raw.Max != nil {
		v.add(key+".max", "an object property has no max; set max on its fields")
	}
	if len(raw.To) > 0 {
		v.add(key+".to", "only a ref property has to")
	}
	if raw.Notify != nil {
		v.add(key+".notify", "only a ref property has notify")
	}
	if n, problem := maxItems(raw, true); problem != "" {
		v.add(key+".max_items", problem)
	} else {
		p.MaxItems = n
	}
	switch {
	case len(raw.Properties) == 0:
		v.add(key+".properties", "an object property declares its fields under properties")
	case len(raw.Properties) > realtimelimits.MaxObjectFields:
		v.add(key+".properties", fmt.Sprintf("an object has at most %d fields", realtimelimits.MaxObjectFields))
	}
	p.Fields = make(map[string]Property, len(raw.Properties))
	for _, name := range sortedKeys(raw.Properties) {
		rf := raw.Properties[name]
		fkey := key + ".properties." + name
		if PropertyType(rf.Type) == PropertyObject {
			v.add(fkey+".type", "an object field cannot be an object; objects nest one level only")
			continue
		}
		f := v.property(fkey, name, &rf, limits)
		if f.Type != "" && !fieldType(f.Type) {
			v.add(fkey+".type", fmt.Sprintf("an object field may be string, integer, boolean, date, datetime "+
				"or an enum type, not %s: the server resolves refs and mentions only in top-level properties",
				f.Type))
			continue
		}
		p.Fields[name] = f
	}
	return p
}
