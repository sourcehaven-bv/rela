package desktopnotify

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Rune limits for rendered text. Native notification centers truncate long
// text anyway; capping it here keeps one huge property from reaching them.
const (
	maxTitleRunes = 120
	maxBodyRunes  = 240
)

// placeholderPattern matches one {{entity.<name>}} placeholder.
var placeholderPattern = regexp.MustCompile(`\{\{\s*entity\.([A-Za-z0-9_-]+)\s*\}\}`)

// Placeholder names that do not read a property.
const (
	fieldID    = "id"
	fieldType  = "type"
	fieldTitle = "title"
)

// template is a parsed title or body. It is a plain substitution: the
// operator's text is never executed, so it cannot call functions.
type template struct {
	// parts alternates literal text and field names: even indexes are
	// literals, odd indexes are fields.
	parts []string
}

// parseTemplate splits src into literals and fields. A "{{" that is not a
// valid placeholder, or a placeholder naming a property the type does not
// declare, is an error.
func parseTemplate(src string, def *metamodel.EntityDef) (template, error) {
	var parts []string
	rest := src
	for {
		loc := placeholderPattern.FindStringSubmatchIndex(rest)
		if loc == nil {
			break
		}
		literal := rest[:loc[0]]
		if strings.Contains(literal, "{{") {
			return template{}, fmt.Errorf("unsupported placeholder in %q: use {{entity.<name>}}", src)
		}
		field := rest[loc[2]:loc[3]]
		if err := checkField(field, def); err != nil {
			return template{}, err
		}
		parts = append(parts, literal, field)
		rest = rest[loc[1]:]
	}
	if strings.Contains(rest, "{{") {
		return template{}, fmt.Errorf("unsupported placeholder in %q: use {{entity.<name>}}", src)
	}
	parts = append(parts, rest)
	return template{parts: parts}, nil
}

func checkField(field string, def *metamodel.EntityDef) error {
	switch field {
	case fieldID, fieldType, fieldTitle:
		return nil
	}
	if _, ok := def.Properties[field]; !ok {
		return fmt.Errorf("unknown property %q in placeholder", field)
	}
	return nil
}

// fields is what a template substitutes from.
type fields struct {
	id, entityType, title string
	props                 map[string]any
}

func (t template) render(f fields) string {
	var b strings.Builder
	for i, p := range t.parts {
		if i%2 == 0 {
			b.WriteString(p)
			continue
		}
		b.WriteString(f.value(p))
	}
	return b.String()
}

func (f fields) value(name string) string {
	switch name {
	case fieldID:
		return f.id
	case fieldType:
		return f.entityType
	case fieldTitle:
		return f.title
	}
	return stringify(f.props[name])
}

// stringify renders a property value as text. A missing value is empty.
func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []any:
		items := make([]string, 0, len(x))
		for _, item := range x {
			items = append(items, stringify(item))
		}
		return strings.Join(items, ", ")
	case []string:
		return strings.Join(x, ", ")
	}
	return fmt.Sprint(v)
}

// plainText removes control characters and caps the length at limit runes.
// A notification title is one line, so it keeps no newlines; a body keeps
// them.
func plainText(s string, limit int, keepNewlines bool) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' && keepNewlines {
			return r
		}
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit-1]) + "…"
}
