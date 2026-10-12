package metamodel

// ResolveWidgetFromType returns the table-cell widget for a property type, as
// used by the data entry app and the migration system.
//
// A package function rather than a method: it reads only m.Types, and
// Metamodel's exported API is capped (plimsoll).
//
// Nil: accepted for m; a nil metamodel declares no custom types, so only the
// built-in types resolve to anything but "text".
func ResolveWidgetFromType(m *Metamodel, propType string) string {
	switch propType {
	case PropertyTypeString:
		return "text"
	case PropertyTypeDate:
		return "date"
	case PropertyTypeDatetime:
		return "datetime"
	case PropertyTypeInteger:
		return "number"
	case PropertyTypeBoolean:
		return "checkbox"
	case PropertyTypeEnum:
		return "select"
	case PropertyTypeRrule:
		return "rrule"
	case PropertyTypeExternalRef:
		return "external-ref"
	default:
		if m == nil {
			return "text"
		}
		if ct, ok := m.Types[propType]; ok && len(ct.Values) > 0 {
			return "select"
		}
		return "text"
	}
}
