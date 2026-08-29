package fsstore

import (
	"path"

	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// fileLayout resolves file keys, plural directory names, and
// schema-declared property order — the path/key/schema half of the
// store's configuration. It is immutable after [New]: every field is
// fixed from [Config] and never written again, so its methods are pure
// functions of configuration and are safe to call with or without the
// store's locks held.
//
// A type of its own rather than more methods on [FSStore], because
// layout resolution needs NOTHING from the store's mutable state — no
// mu, no index maps, no observers. Hanging these helpers off FSStore
// buried a pure function group inside a god-object; the linter counts
// methods per type, and so does anyone reading FSStore's API.
type fileLayout struct {
	// entitiesKey and relationsKey are root-relative forward-slash
	// keys for the standard subtrees (e.g. "entities", "relations").
	entitiesKey  string
	relationsKey string

	// schemas maps entity types to their storage-relevant
	// configuration (plural directory name + property order). Complete
	// by construction: [New] rejects an empty map.
	schemas map[string]store.EntityTypeSchema

	// rooted resolves keys to absolute paths for the watcher and the
	// self-echo LRU. It is used read-only here; all file I/O stays on
	// FSStore.
	rooted *storage.RootedFS
}

// entityFileKey returns the key for an entity file:
// "<entitiesKey>/<plural>/<id>.md" — forward slashes, no leading slash.
func (l fileLayout) entityFileKey(entityType, id string) string {
	plural := entityType + "s"
	if schema, ok := l.schemas[entityType]; ok && schema.Plural != "" {
		plural = schema.Plural
	}
	return path.Join(l.entitiesKey, plural, id+".md")
}

func (l fileLayout) relationFileKey(from, relType, to string) string {
	return path.Join(l.relationsKey, from+"--"+relType+"--"+to+".md")
}

// propertyOrder returns the property order for an entity type, if configured.
func (l fileLayout) propertyOrder(entityType string) []string {
	if schema, ok := l.schemas[entityType]; ok {
		return schema.PropertyOrder
	}
	return nil
}

// absPath resolves a key to an absolute path. Used by the watcher
// (which needs absolute paths for fsnotify) and the self-echo LRU
// interaction points, where paths must match what SafeFS.OnPostWrite
// observes.
//
// Returns "" on resolve failure — keys constructed from configured
// fields should always resolve, but upstream validators
// (storeutil.ValidateID) don't cover all cases RootedFS rejects
// (e.g. Windows reserved names). A resolve failure here means no
// file was ever written under that key, so the LRU can safely no-op
// a Forget and the watcher can safely skip-setup.
func (l fileLayout) absPath(key string) string {
	abs, err := l.rooted.AbsPath(key)
	if err != nil {
		return ""
	}
	return abs
}

// buildPluralToTypeMap builds a reverse map from plural directory names to entity types.
func (l fileLayout) buildPluralToTypeMap() map[string]string {
	m := make(map[string]string, len(l.schemas))
	for typ, schema := range l.schemas {
		if schema.Plural != "" {
			m[schema.Plural] = typ
		}
	}
	return m
}

// resolveEntityType maps a plural directory name back to the entity type.
// Returns "" when the directory does not map to any metamodel-declared
// type — callers skip such directories.
//
// The schema's Plural field is canonical: app.buildSchemas resolves it
// via metamodel.EntityDef.GetPlural so fsstore does not have to
// re-guess via "+s" at call time.
func (l fileLayout) resolveEntityType(dirName string, pluralToType map[string]string) string {
	if typ, ok := pluralToType[dirName]; ok {
		return typ
	}
	return ""
}
