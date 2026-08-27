package entitymanager

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ComputedWriteError reports an attempt to author a materialized computed
// property. Callers may errors.As this into a surface-specific 422 response.
type ComputedWriteError struct {
	EntityType string
	Properties []string
}

func (e *ComputedWriteError) Error() string {
	return fmt.Sprintf("computed properties are read-only for entity type %q: %v", e.EntityType, e.Properties)
}

func rejectComputedPresent(deps Deps, entityType string, props map[string]any) error {
	var names []string
	for name := range props {
		if deps.Computed.IsComputed(entityType, name) {
			names = append(names, name)
		}
	}
	return computedWriteError(entityType, names)
}

func rejectComputedPatch(deps Deps, entityType string, props map[string]any, unset []string) error {
	var names []string
	for name := range props {
		if deps.Computed.IsComputed(entityType, name) {
			names = append(names, name)
		}
	}
	for _, name := range unset {
		if deps.Computed.IsComputed(entityType, name) {
			names = append(names, name)
		}
	}
	return computedWriteError(entityType, names)
}

func rejectComputedChanges(deps Deps, old, updated *entity.Entity) error {
	var names []string
	def, ok := deps.Meta.GetEntityDef(updated.Type)
	if !ok {
		return nil
	}
	for name, pd := range def.Properties {
		if pd.Computed == "" {
			continue
		}
		ov, ook := old.Properties[name]
		nv, nok := updated.Properties[name]
		if ook != nok || !reflect.DeepEqual(ov, nv) {
			names = append(names, name)
		}
	}
	return computedWriteError(updated.Type, names)
}

func computedWriteError(entityType string, names []string) error {
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	names = slicesCompact(names)
	return &ComputedWriteError{EntityType: entityType, Properties: names}
}

func slicesCompact(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}

func stringMapAny(in map[string]string) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
