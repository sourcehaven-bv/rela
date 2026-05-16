package attachment

import (
	"mime"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// findFileProperty returns the alphabetically-first file-type
// property name on an entity definition, or "" if none is declared.
//
// Alphabetical order is the only deterministic order available —
// Go map iteration is randomized, and the metamodel doesn't preserve
// declaration order on parsed entity definitions. Callers that care
// which file property gets the attachment must pass `property`
// explicitly.
func findFileProperty(entityDef *metamodel.EntityDef) string {
	names := make([]string, 0, len(entityDef.Properties))
	for name, prop := range entityDef.Properties {
		if prop.Type == metamodel.PropertyTypeFile {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return names[0]
}

// contentTypeForName infers a MIME type from a filename extension.
// Falls back to application/octet-stream — browsers render that as a
// download prompt, which is the right default for unknown types.
func contentTypeForName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return "application/octet-stream"
	}
	if mt := mime.TypeByExtension(ext); mt != "" {
		return mt
	}
	return "application/octet-stream"
}
