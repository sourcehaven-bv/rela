package cli

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// resolveEntityType resolves a type name (alias, plural) to its
// canonical name and definition. Lifted from
// workspace.(*Workspace).ResolveEntityType so CLI doesn't pull a
// method off the workspace bundle for a pure metamodel operation.
func resolveEntityType(meta *metamodel.Metamodel, typeName string) (string, *metamodel.EntityDef, error) {
	resolved := meta.ResolveAlias(strings.TrimSpace(typeName))
	if def, ok := meta.GetEntityDef(resolved); ok {
		return resolved, def, nil
	}

	suffixes := []string{"ies", "es", "s"}
	replacements := []string{"y", "", ""}
	for i, suffix := range suffixes {
		if strings.HasSuffix(typeName, suffix) {
			singular := strings.TrimSuffix(typeName, suffix) + replacements[i]
			resolved = meta.ResolveAlias(singular)
			if def, ok := meta.GetEntityDef(resolved); ok {
				return resolved, def, nil
			}
		}
	}

	return "", nil, fmt.Errorf("unknown entity type: %s", typeName)
}
