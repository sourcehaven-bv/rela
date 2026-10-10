package configedit

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// yamlPaths lists every key path a configuration struct can hold, with "*"
// for a map key and "[]" for a list item. A path that stops at a type with
// its own YAML decoding ends there.
func yamlPaths(t reflect.Type) []string {
	seen := map[reflect.Type]bool{}
	var out []string
	var walk func(t reflect.Type, prefix string)
	walk = func(t reflect.Type, prefix string) {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		switch t.Kind() {
		case reflect.Map:
			walk(t.Elem(), prefix+".*")
			return
		case reflect.Slice, reflect.Array:
			walk(t.Elem(), prefix+".[]")
			return
		case reflect.Struct:
		default:
			out = append(out, prefix)
			return
		}
		if seen[t] {
			out = append(out, prefix+" (recursive "+t.Name()+")")
			return
		}
		seen[t] = true
		defer delete(seen, t)
		fields := 0
		for f := range t.Fields() {
			tag := f.Tag.Get("yaml")
			name := strings.Split(tag, ",")[0]
			if tag == "-" || (!f.IsExported() && !f.Anonymous) {
				continue
			}
			if strings.Contains(tag, "inline") || (f.Anonymous && name == "") {
				walk(f.Type, prefix)
				fields++
				continue
			}
			if name == "" {
				name = strings.ToLower(f.Name)
			}
			walk(f.Type, prefix+"."+name)
			fields++
		}
		if fields == 0 {
			out = append(out, prefix)
		}
	}
	walk(t, "")
	sort.Strings(out)
	for i := range out {
		out[i] = strings.TrimPrefix(out[i], ".")
	}
	return out
}

func TestDumpPaths(t *testing.T) {
	if os.Getenv("DUMP_PATHS") == "" {
		t.Skip()
	}
	for _, p := range yamlPaths(reflect.TypeFor[metamodel.Metamodel]()) {
		t.Log("schema " + p)
	}
	for _, p := range yamlPaths(reflect.TypeFor[dataentryconfig.Config]()) {
		t.Log("data-entry " + p)
	}
}
