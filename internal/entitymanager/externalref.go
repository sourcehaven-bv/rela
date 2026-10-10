package entitymanager

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// External refs link an entity to its counterpart in another system, and a
// sync connector trusts them to find that counterpart (TKT-SM20FG). They are
// therefore not writable from interactive surfaces (data-entry, MCP, CLI
// `-P`, forms): only a write that sets WriteExternalRefs on its options may
// change one. The Lua write bindings set it only in runtimes built with
// lua.WithExternalRefWrites (operator-authored scripts); migrations and
// imports write below the manager. A history restore carries the live
// values (see [CarryFileValues]), so it changes none, and a recreate of a
// deleted entity refuses refs unless its caller opts in ([Recreator]).
// System writes that no field gate sees (automation `set:`, cascade
// `create_entity` properties) never write a ref: the metamodel refuses
// such an automation at load, and [rejectExternalRefPresent] backs that up
// at write time.

// rejectExternalRefCreate refuses a create that sets an external ref unless
// allowed.
func rejectExternalRefCreate(meta *metamodel.Metamodel, entityType string, props map[string]any, allowed bool) error {
	if allowed {
		return nil
	}
	var names []string
	for _, name := range metamodel.ExternalRefPropertyNames(meta, entityType) {
		if !metamodel.IsEmptyValue(props[name]) {
			names = append(names, name)
		}
	}
	return externalRefWriteError(names)
}

// rejectExternalRefPresent refuses an automation-derived property set that
// names an external ref of entityType at all, even to clear it: such a
// write has no caller to grant ref writes.
func rejectExternalRefPresent(meta *metamodel.Metamodel, entityType string, props map[string]any) error {
	var names []string
	for _, name := range metamodel.ExternalRefPropertyNames(meta, entityType) {
		if _, ok := props[name]; ok {
			names = append(names, name)
		}
	}
	return externalRefWriteError(names)
}

// rejectExternalRefChanges refuses a save that changes an external ref of
// old unless allowed. An unchanged value, including an absent one on both
// sides, passes, so a whole-entity save that carries the ref is not refused.
func rejectExternalRefChanges(meta *metamodel.Metamodel, old, updated *entity.Entity, allowed bool) error {
	if allowed {
		return nil
	}
	var names []string
	for _, name := range metamodel.ExternalRefPropertyNames(meta, updated.Type) {
		ov, nv := old.Properties[name], updated.Properties[name]
		if metamodel.IsEmptyValue(ov) && metamodel.IsEmptyValue(nv) {
			continue
		}
		if !reflect.DeepEqual(ov, nv) {
			names = append(names, name)
		}
	}
	return externalRefWriteError(names)
}

// externalRefWriteError is the hard 422 for names, or nil when empty.
func externalRefWriteError(names []string) error {
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	errs := make([]*metamodel.ValidationError, 0, len(names))
	for _, name := range names {
		errs = append(errs, &metamodel.ValidationError{
			Type:     metamodel.ValidationErrorExternalRef,
			Property: name,
			Message: fmt.Sprintf(
				"property %q is an external ref; only scripts, migrations and imports may write it", name),
		})
	}
	return newValidationError(errs)
}
