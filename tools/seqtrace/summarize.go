package seqtrace

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// maxValue caps one summarized value. Diagrams show values on arrow labels,
// so a summary identifies a value; it does not reproduce it.
const maxValue = 48

// secretName matches parameter and function names whose values must not
// reach a trace file. A matching parameter is redacted; a matching function
// has all its arguments and results redacted, which covers a getter such as
// resolvePassword. It is a heuristic: see the README on handling traces.
var (
	secretName = regexp.MustCompile(`(?i)pass|secret|token|cred|auth|jwt|bearer|dsn|cookie|key$`)
	// secretFunc is narrower: "auth" would hide every ACL decision.
	secretFunc = regexp.MustCompile(`(?i)pass|secret|token|cred|jwt|bearer|dsn|cookie|apikey`)
)

const redacted = "‹redacted›"

// opaque wraps a value whose declared type is an interface with no methods
// (any). Such values are usually data (property values, decoded JSON), so
// only their type is recorded.
type opaque struct{ v any }

// Opaque is injected around arguments and results declared as any.
func Opaque(v any) any { return opaque{v} }

// idFields are the struct fields that identify a value, in display order.
var idFields = []string{"Type", "ID", "Face", "Name", "Kind", "Op", "From", "To", "Path"}

// summarizeArgs turns the injected name/value pairs into "name=summary"
// strings. It skips context values, which carry no data worth drawing.
func summarizeArgs(fn string, pairs []any) []string {
	secretFn := secretFunc.MatchString(fn)
	var out []string
	for i := 0; i+1 < len(pairs); i += 2 {
		name, _ := pairs[i].(string)
		v := pairs[i+1]
		if _, ok := v.(context.Context); ok {
			continue
		}
		if secretFn || secretName.MatchString(name) {
			out = append(out, name+"="+redacted)
			continue
		}
		out = append(out, name+"="+summarize(v))
	}
	return out
}

func summarizeResults(fn string, vals []any) []string {
	secretFn := secretFunc.MatchString(fn)
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if secretFn {
			out = append(out, redacted)
			continue
		}
		out = append(out, summarize(v))
	}
	return out
}

// summarize renders v in at most maxValue characters. It never calls a
// String method (which may lock or allocate unboundedly); it does call
// Error, because an error's message is usually the point.
func summarize(v any) (s string) {
	defer func() {
		if recover() != nil {
			s = "?"
		}
	}()
	switch x := v.(type) {
	case nil:
		return "nil"
	case opaque:
		if x.v == nil {
			return "nil"
		}
		return reflect.TypeOf(x.v).String()
	case error:
		if rv := reflect.ValueOf(x); rv.Kind() == reflect.Pointer && rv.IsNil() {
			return "nil"
		}
		return truncate("err: " + oneLine(x.Error()))
	case string:
		return strconv.Quote(truncate(x))
	case []byte:
		return fmt.Sprintf("[]byte len=%d", len(x))
	}
	return truncate(summarizeValue(reflect.ValueOf(v)))
}

func summarizeValue(rv reflect.Value) string {
	t := rv.Type()
	switch rv.Kind() {
	case reflect.Pointer:
		if rv.IsNil() {
			return "nil"
		}
		return "*" + summarizeValue(rv.Elem())
	case reflect.Interface:
		if rv.IsNil() {
			return "nil"
		}
		return summarizeValue(rv.Elem())
	case reflect.Struct:
		return summarizeStruct(rv)
	case reflect.Slice, reflect.Array, reflect.Map:
		return fmt.Sprintf("%s len=%d", t, rv.Len())
	case reflect.Func:
		return "func"
	case reflect.Chan:
		return "chan"
	case reflect.String:
		if t.Name() == "string" {
			return strconv.Quote(rv.String())
		}
		return t.Name() + "(" + strconv.Quote(rv.String()) + ")"
	// Format from the reflect value: fmt would call a String method.
	case reflect.Bool:
		return strconv.FormatBool(rv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
	default:
		return t.String()
	}
}

// summarizeStruct shows the type and the fields that identify the value, so
// an entity reads as entity.Entity{Type:task ID:TSK-0001}.
func summarizeStruct(rv reflect.Value) string {
	t := rv.Type()
	var parts []string
	for _, name := range idFields {
		f, ok := t.FieldByName(name)
		if !ok || !f.IsExported() || len(f.Index) != 1 {
			continue
		}
		fv := rv.Field(f.Index[0])
		if fv.Kind() != reflect.String || fv.String() == "" {
			continue
		}
		parts = append(parts, name+":"+oneLine(fv.String()))
	}
	if len(parts) == 0 {
		return t.String()
	}
	return t.String() + "{" + strings.Join(parts, " ") + "}"
}

// oneLine keeps unquoted text on one line, so it cannot start a new
// statement in a diagram.
func oneLine(s string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`, "\x00", `\0`).Replace(s)
}

func truncate(s string) string {
	if len(s) <= maxValue {
		return s
	}
	// Cut on a rune boundary.
	cut := maxValue
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}
