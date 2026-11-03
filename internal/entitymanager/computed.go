package entitymanager

import (
	"fmt"
	"maps"
	"reflect"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// ComputedWriteError reports an attempt to author a read-only property: a
// materialized computed property, or a roll-up (TKT-6I2OPG), which is computed
// on read and never stored. Callers may errors.As this into a
// surface-specific 422 response.
type ComputedWriteError struct {
	EntityType string
	Properties []string
}

func (e *ComputedWriteError) Error() string {
	return fmt.Sprintf("computed and roll-up properties are read-only for entity type %q: %v",
		e.EntityType, e.Properties)
}

// readOnly reports whether name is a computed or roll-up property of
// entityType.
func readOnly(deps Deps, entityType, name string) bool {
	if deps.Computed.IsComputed(entityType, name) {
		return true
	}
	def, ok := deps.Meta.GetEntityDef(entityType)
	if !ok {
		return false
	}
	pd, ok := def.Properties[name]
	return ok && pd.Rollup != nil
}

func rejectComputedPresent(deps Deps, entityType string, props map[string]any) error {
	var names []string
	for name := range props {
		if readOnly(deps, entityType, name) {
			names = append(names, name)
		}
	}
	return computedWriteError(entityType, names)
}

func rejectComputedPatch(deps Deps, entityType string, props map[string]any, unset []string) error {
	var names []string
	for name := range props {
		if readOnly(deps, entityType, name) {
			names = append(names, name)
		}
	}
	for _, name := range unset {
		if readOnly(deps, entityType, name) {
			names = append(names, name)
		}
	}
	return computedWriteError(entityType, names)
}

// rejectComputedChanges refuses a whole-entity update that changes a computed
// property. A roll-up is instead pinned to what is stored: a whole-entity
// save round-trips the read, which carries the computed roll-up value, and
// that value is never stored. Pinning (as for file values) keeps whatever the
// row holds under the name, so the save neither stores the read's value nor
// erases a hand-edited one; reads never serve it.
func rejectComputedChanges(deps Deps, old, updated *entity.Entity) error {
	var names []string
	def, ok := deps.Meta.GetEntityDef(updated.Type)
	if !ok {
		return nil
	}
	cloned := false
	for name, pd := range def.Properties {
		if pd.Rollup != nil {
			cloned = pinStored(old, updated, name, cloned)
			continue
		}
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

// pinStored sets updated's value for name to old's, removing it when old has
// none. cloned reports whether updated.Properties is already a private copy;
// the result reports the same after the call.
func pinStored(old, updated *entity.Entity, name string, cloned bool) bool {
	ov, ook := old.Properties[name]
	nv, nok := updated.Properties[name]
	if ook == nok && reflect.DeepEqual(ov, nv) {
		return cloned
	}
	if !cloned {
		updated.Properties = maps.Clone(updated.Properties)
	}
	if !ook {
		delete(updated.Properties, name)
		return true
	}
	if updated.Properties == nil {
		updated.Properties = map[string]any{}
	}
	updated.Properties[name] = ov
	return true
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
