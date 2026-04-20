package fsstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const indexFile = "fsstore-index.json"

// persistedIndex is the on-disk format for the cached store state.
type persistedIndex struct {
	// EntitiesDirMtime is the latest mtime across all entity type subdirectories.
	EntitiesDirMtime time.Time `json:"entities_dir_mtime"`
	// RelationsDirMtime is the mtime of the relations directory.
	RelationsDirMtime time.Time `json:"relations_dir_mtime"`

	Entities  map[string]indexedEntity   `json:"entities"`  // id → meta
	Relations map[string]indexedRelation `json:"relations"` // key → meta

	// PropCacheMtime is the newest entity file mtime when the prop cache was built.
	PropCacheMtime time.Time                 `json:"prop_cache_mtime"`
	PropCache      map[string]map[string]int `json:"prop_cache"` // property → value → count
}

type indexedEntity struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type indexedRelation struct {
	From string `json:"from"`
	Type string `json:"type"`
	To   string `json:"to"`
}

// loadPersistedIndex reads the index from disk. Returns nil if missing or corrupt.
func (s *FSStore) loadPersistedIndex() *persistedIndex {
	if s.cacheDir == "" {
		return nil
	}
	data, err := s.readDataFile(filepath.Join(s.cacheDir, indexFile))
	if err != nil {
		return nil
	}
	var idx persistedIndex
	if json.Unmarshal(data, &idx) != nil {
		return nil
	}
	return &idx
}

// savePersistedIndex writes the current index state to disk.
func (s *FSStore) savePersistedIndex() error {
	if s.cacheDir == "" {
		return nil
	}

	newestFile := s.newestEntityFileMtime()

	idx := persistedIndex{
		EntitiesDirMtime:  s.entitiesDirMtime(),
		RelationsDirMtime: s.relationsDirMtime(),
		Entities:          make(map[string]indexedEntity, len(s.entities)),
		Relations:         make(map[string]indexedRelation, len(s.relations)),
		PropCacheMtime:    newestFile,
		PropCache:         s.propCache,
	}

	for id, meta := range s.entities {
		idx.Entities[id] = indexedEntity{ID: id, Type: meta.Type}
	}
	for key, meta := range s.relations {
		idx.Relations[key] = indexedRelation(meta)
	}

	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	if mkdirErr := s.dirs.MkdirAll(s.cacheDir, 0o755); mkdirErr != nil {
		return mkdirErr
	}
	return s.bytes.WriteFile(filepath.Join(s.cacheDir, indexFile), data, 0o644)
}

// syncIndex reconciles all in-memory state with the filesystem:
//  1. Entity index: dir mtime check → restore from cache or rescan dirs
//  2. Relation index: dir mtime check → restore from cache or rescan dirs
//  3. Scan all entity files for newest mtime (stat only, no reads)
//  4. Prop cache: compare newest mtime → restore from cache or rebuild
func (s *FSStore) syncIndex() error {
	cached := s.loadPersistedIndex()

	if err := s.syncEntities(cached); err != nil {
		return err
	}
	if err := s.syncRelations(cached); err != nil {
		return err
	}

	newestFile := s.newestEntityFileMtime()

	if cached != nil && cached.PropCache != nil && !newestFile.After(cached.PropCacheMtime) {
		s.propCache = cached.PropCache
		return nil
	}
	return s.rebuildPropCache()
}

// syncEntities builds the entity index from directory structure.
// If the cached index is fresh (dir mtime unchanged), restores from cache.
// Otherwise walks entity type dirs: dir name → type, filename → ID.
func (s *FSStore) syncEntities(cached *persistedIndex) error {
	currentMtime := s.entitiesDirMtime()

	if cached != nil && cached.Entities != nil && currentMtime.Equal(cached.EntitiesDirMtime) {
		for id, ie := range cached.Entities {
			s.entities[id] = entityMeta(ie)
			s.entityOrder = append(s.entityOrder, id)
		}
		sortStrings(s.entityOrder)
		return nil
	}

	return s.scanEntityDirs()
}

// syncRelations builds the relation index from filenames.
// If the cached index is fresh (dir mtime unchanged), restores from cache.
// Otherwise lists the relations dir and parses FROM--TYPE--TO from filenames.
func (s *FSStore) syncRelations(cached *persistedIndex) error {
	currentMtime := s.relationsDirMtime()

	if cached != nil && cached.Relations != nil && currentMtime.Equal(cached.RelationsDirMtime) {
		for key, ir := range cached.Relations {
			s.relations[key] = relationMeta(ir)
			s.relationOrder = append(s.relationOrder, key)
		}
		sortStrings(s.relationOrder)
		return nil
	}

	return s.scanRelationDir()
}

// rebuildPropCache reads every entity file to repopulate the property cache.
// Called when the cached cache is stale (newer entity files exist on disk).
func (s *FSStore) rebuildPropCache() error {
	s.propCache = make(map[string]map[string]int)
	for _, meta := range s.entities {
		e, err := s.loadEntity(meta.ID, meta.Type)
		if err != nil {
			continue
		}
		addEntityToCache(s.propCache, e)
	}
	return nil
}

// scanEntityDirs walks the entity type directories concurrently and
// populates the index from directory structure alone (no file reads).
func (s *FSStore) scanEntityDirs() error {
	typeDirs, err := s.dirs.ReadDir(s.entitiesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	pluralToType := s.buildPluralToTypeMap()

	type result struct {
		entries []entityMeta
	}

	// Scan each type directory concurrently.
	results := make([]result, len(typeDirs))
	var wg sync.WaitGroup
	for i, typeDir := range typeDirs {
		if !typeDir.IsDir() {
			continue
		}
		wg.Add(1)
		go func(idx int, dirName string) {
			defer wg.Done()
			entityType := s.resolveEntityType(dirName, pluralToType)
			files, readErr := s.dirs.ReadDir(filepath.Join(s.entitiesDir, dirName))
			if readErr != nil {
				return
			}
			var entries []entityMeta
			for _, f := range files {
				if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
					continue
				}
				id := strings.TrimSuffix(f.Name(), ".md")
				entries = append(entries, entityMeta{ID: id, Type: entityType})
			}
			results[idx] = result{entries: entries}
		}(i, typeDir.Name())
	}
	wg.Wait()

	// Merge results into the index.
	for _, r := range results {
		for _, meta := range r.entries {
			s.entities[meta.ID] = meta
			s.entityOrder = append(s.entityOrder, meta.ID)
		}
	}

	sortStrings(s.entityOrder)
	return nil
}

// scanRelationDir lists the relations directory and parses relation keys
// from filenames (FROM--TYPE--TO.md). No file reads needed.
func (s *FSStore) scanRelationDir() error {
	files, err := s.dirs.ReadDir(s.relationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(f.Name(), ".md")
		from, relType, to := parseRelationFilename(name)
		if from == "" || relType == "" || to == "" {
			continue
		}
		key := from + "--" + relType + "--" + to
		s.relations[key] = relationMeta{From: from, Type: relType, To: to}
		s.relationOrder = append(s.relationOrder, key)
	}

	sortStrings(s.relationOrder)
	return nil
}

// parseRelationFilename splits "FROM--TYPE--TO" into its three parts.
// Returns empty strings if the format is invalid.
func parseRelationFilename(name string) (from, relType, to string) {
	i := strings.Index(name, "--")
	if i < 1 {
		return "", "", ""
	}
	from = name[:i]
	rest := name[i+2:]

	j := strings.LastIndex(rest, "--")
	if j < 1 {
		return "", "", ""
	}
	relType = rest[:j]
	to = rest[j+2:]
	if to == "" {
		return "", "", ""
	}
	return from, relType, to
}

// buildPluralToTypeMap builds a reverse map from plural directory names to entity types.
func (s *FSStore) buildPluralToTypeMap() map[string]string {
	m := make(map[string]string, len(s.schemas))
	for typ, schema := range s.schemas {
		if schema.Plural != "" {
			m[schema.Plural] = typ
		}
	}
	return m
}

// resolveEntityType maps a plural directory name back to the entity type.
// Uses the schema's plural mapping if available, otherwise strips trailing "s".
func (s *FSStore) resolveEntityType(dirName string, pluralToType map[string]string) string {
	if typ, ok := pluralToType[dirName]; ok {
		return typ
	}
	return strings.TrimSuffix(dirName, "s")
}

// newestEntityFileMtime returns the newest mtime across all entity files.
// Uses stat only — no file reads.
func (s *FSStore) newestEntityFileMtime() time.Time {
	var newest time.Time
	for _, meta := range s.entities {
		path := s.entityFilePath(meta.Type, meta.ID)
		if info, err := s.dirs.Stat(path); err == nil {
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
		}
	}
	return newest
}

// newestRelationFileMtime returns the newest mtime across all relation files.
// Uses stat only — no file reads.
func (s *FSStore) newestRelationFileMtime() time.Time {
	var newest time.Time
	for _, meta := range s.relations {
		path := s.relationFilePath(meta.From, meta.Type, meta.To)
		if info, err := s.dirs.Stat(path); err == nil {
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
		}
	}
	return newest
}

// entitiesDirMtime returns the latest mtime across all entity type subdirectories.
func (s *FSStore) entitiesDirMtime() time.Time {
	var latest time.Time

	info, err := s.dirs.Stat(s.entitiesDir)
	if err != nil {
		return latest
	}
	latest = info.ModTime()

	entries, err := s.dirs.ReadDir(s.entitiesDir)
	if err != nil {
		return latest
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if info, err := entry.Info(); err == nil {
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
	}
	return latest
}

// relationsDirMtime returns the mtime of the relations directory.
func (s *FSStore) relationsDirMtime() time.Time {
	info, err := s.dirs.Stat(s.relationsDir)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
