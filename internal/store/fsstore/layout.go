package fsstore

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// fileLayout resolves file keys, plural directory names, and
// schema-declared property order — the path/key/schema half of the
// store's configuration. The keys and the rooted FS are fixed in [New].
// The schemas are published through an atomic pointer that
// [FSStore.SetSchemas] replaces, so its methods are safe to call with
// or without the store's locks held, and copies of a fileLayout (the
// store's and the codec's) all see the same generation.
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

	// schemas is the current generation of entity types. Never nil
	// and never empty: [New] stores the first one and rejects an
	// empty map.
	schemas *atomic.Pointer[schemaSet]

	// rooted resolves keys to absolute paths for the watcher and the
	// self-echo LRU. It is used read-only here; all file I/O stays on
	// FSStore.
	rooted *storage.RootedFS
}

// schemaSet is one published generation of the store's entity types.
// It is read-only once published; a schema change publishes a new set.
type schemaSet struct {
	// declared holds the types the current schema declares. Every
	// entry has a non-empty Plural (see normalizeSchemas). Writes are
	// gated on it, the directory scan and the watcher map plurals
	// through it, and every type in the live index is in it.
	declared map[string]store.EntityTypeSchema
}

// normalizeSchemas validates a caller's schema map and returns a copy in
// which every type has a Plural. An empty Plural becomes "<type>s", so
// the directory a write uses and the directory the scan maps back to a
// type come from the same value.
func normalizeSchemas(in map[string]store.EntityTypeSchema) (map[string]store.EntityTypeSchema, error) {
	// Without schemas, fsstore cannot map plural directory names back
	// to entity types, and inaccessible-entity shells (for git-crypt
	// encrypted files) cannot enumerate their declared properties. An
	// empty map always indicates a bootstrap-ordering bug.
	if len(in) == 0 {
		return nil, errors.New("fsstore: schemas must be populated from the metamodel; " +
			"an empty map indicates a bootstrap-ordering bug")
	}
	out := make(map[string]store.EntityTypeSchema, len(in))
	owner := make(map[string]string, len(in))
	for _, typ := range slices.Sorted(maps.Keys(in)) {
		schema := in[typ]
		if typ == "" {
			return nil, errors.New("fsstore: schemas declare an entity type with an empty name")
		}
		if schema.Plural == "" {
			schema.Plural = typ + "s"
		}
		// The plural is joined under entitiesKey, and a key is cleaned
		// before RootedFS checks it, so "../relations" would write into
		// relations/. Only a single directory name is allowed.
		if err := storage.ValidateDirSegment(schema.Plural); err != nil {
			return nil, fmt.Errorf("fsstore: entity type %q: plural %q is not a usable directory name: %w", typ, schema.Plural, err)
		}
		// Two types in one directory would both write there, and the scan
		// would read every file back as one of them. Compared without case,
		// because the default macOS and Windows filesystems ignore it.
		folded := strings.ToLower(schema.Plural)
		if other, ok := owner[folded]; ok {
			return nil, fmt.Errorf("fsstore: entity types %q and %q share the directory %q", other, typ, schema.Plural)
		}
		owner[folded] = typ
		out[typ] = schema
	}
	return out, nil
}

// errUnknownType is the error a write of an undeclared entity type
// returns. The type is quoted so an empty one is visible. The wording
// matches the entitymanager's, so the error reads the same on both
// layers.
func errUnknownType(entityType string) error {
	return fmt.Errorf("fsstore: unknown entity type %q", entityType)
}

// declared reports whether the current schema declares entityType.
// Only a declared type can be written: the file of an undeclared type
// would land in a directory the scan on the next open skips, so the
// entity would be lost without an error.
func (l fileLayout) declared(entityType string) bool {
	_, ok := l.schemas.Load().declared[entityType]
	return ok
}

// entityFileKey returns the key for an entity file:
// "<entitiesKey>/<plural>/<id>.md" — forward slashes, no leading slash.
//
// It does not check the type. Writes check it first ([fileLayout.declared]),
// and the other callers act on index entries. Every indexed type is
// declared: the open scan and the watcher index only files in the
// directory of a declared type, and [FSStore.SetSchemas] drops the
// entries of a type it removes. Soft-deleted families of a removed type
// stay marked, and the soft-delete operations skip them (familyDeclared).
// Only the mention index build still asks for their keys; whatever the
// "+s" fallback finds there is ignored, because the entity is not live.
// The fallback exists so this function cannot fail.
func (l fileLayout) entityFileKey(entityType, id string) string {
	plural := entityType + "s"
	if schema, ok := l.schemas.Load().declared[entityType]; ok {
		plural = schema.Plural
	}
	return path.Join(l.entitiesKey, plural, id+".md")
}

// relationFileKeyMeta returns the key for the relation an index meta
// describes; the FROM slot carries the tail face (relKey).
func (l fileLayout) relationFileKeyMeta(m relationMeta) string {
	return path.Join(l.relationsKey, m.key()+".md")
}

// propertyOrder returns the property order for an entity type, if configured.
func (l fileLayout) propertyOrder(entityType string) []string {
	if schema, ok := l.schemas.Load().declared[entityType]; ok {
		return schema.PropertyOrder
	}
	return nil
}

// absPath resolves a key to an absolute path. Used by the watcher
// (which needs absolute paths for the file watcher) and the self-echo LRU
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

// buildPluralToTypeMap builds a reverse map from plural directory names
// to the declared entity types.
func (l fileLayout) buildPluralToTypeMap() map[string]string {
	declared := l.schemas.Load().declared
	m := make(map[string]string, len(declared))
	for typ, schema := range declared {
		m[schema.Plural] = typ
	}
	return m
}

// resolveEntityType maps a plural directory name back to the entity type.
// Returns "" when the directory does not map to any metamodel-declared
// type — callers skip such directories.
//
// The schema's Plural field is canonical: app.buildSchemas resolves it
// via metamodel.EntityDef.GetPlural, and normalizeSchemas fills it for
// any other caller, so fsstore never re-guesses via "+s" at call time.
func (l fileLayout) resolveEntityType(dirName string, pluralToType map[string]string) string {
	if typ, ok := pluralToType[dirName]; ok {
		return typ
	}
	return ""
}
