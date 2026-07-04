package cli

import (
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// resolveEntityType resolves a type name (alias, plural) to its
// canonical name, erroring if no such type exists. Lifted from
// workspace.(*Workspace).ResolveEntityType so CLI doesn't pull a
// method off the workspace bundle for a pure metamodel operation.
func resolveEntityType(meta *metamodel.Metamodel, typeName string) (string, error) {
	resolved := meta.ResolveAlias(strings.TrimSpace(typeName))
	if _, ok := meta.GetEntityDef(resolved); ok {
		return resolved, nil
	}

	suffixes := []string{"ies", "es", "s"}
	replacements := []string{"y", "", ""}
	for i, suffix := range suffixes {
		if strings.HasSuffix(typeName, suffix) {
			singular := strings.TrimSuffix(typeName, suffix) + replacements[i]
			resolved = meta.ResolveAlias(singular)
			if _, ok := meta.GetEntityDef(resolved); ok {
				return resolved, nil
			}
		}
	}

	return "", fmt.Errorf("unknown entity type: %s", typeName)
}
