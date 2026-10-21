package fsstore

import (
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// syncIndex builds the entity and relation index from the directory
// listings. Every open scans, so the index cannot go stale. The scan lists
// directories and reads no file bodies.
func (s *FSStore) syncIndex() error {
	if err := s.scanEntityDirs(); err != nil {
		return err
	}
	return s.scanRelationDir()
}

// scanEntityDirs walks the entity type directories concurrently and
// populates the index from directory structure alone (no file reads).
func (s *FSStore) scanEntityDirs() error {
	typeDirs, err := s.rooted.ReadDir(s.layout.entitiesKey)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	pluralToType := s.layout.buildPluralToTypeMap()

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
			entityType := s.layout.resolveEntityType(dirName, pluralToType)
			if entityType == "" {
				// Directory does not map to a metamodel-declared type.
				// Skipping prevents indexing files for types we cannot
				// reason about (no schema, no property order, no
				// reliable inaccessible-shell). Schema drift sees the
				// orphan directory as invisible until cleaned up.
				return
			}
			files, readErr := s.rooted.ReadDir(path.Join(s.layout.entitiesKey, dirName))
			if readErr != nil {
				return
			}
			var entries []entityMeta
			for _, f := range files {
				if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
					continue
				}
				stem := strings.TrimSuffix(f.Name(), ".md")
				// The stem is the state key: bare id, or "id@face"
				// (TKT-DOFYR1). A stem the codec rejects is skipped
				// with a warning, never a hard error — the load path
				// tolerates hand-edited layouts; analyze surfaces them.
				id, ptr, refErr := entity.ParseStateRef(stem)
				if refErr != nil {
					slog.Warn("fsstore: skipping entity file with invalid name",
						"file", f.Name(), "dir", dirName, "error", refErr)
					continue
				}
				entries = append(entries, entityMeta{ID: id, Type: entityType, Face: ptr})
			}
			results[idx] = result{entries: entries}
		}(i, typeDir.Name())
	}
	wg.Wait()

	// Merge results into the index.
	for _, r := range results {
		for _, meta := range r.entries {
			key := stateKey(meta.ID, meta.Face)
			s.entities[key] = meta
			s.entityOrder = append(s.entityOrder, key)
		}
	}

	sortStateKeys(s.entityOrder)
	return nil
}

// scanRelationDir lists the relations directory and parses relation keys
// from filenames (FROM--TYPE--TO.md). No file reads needed.
func (s *FSStore) scanRelationDir() error {
	files, err := s.rooted.ReadDir(s.layout.relationsKey)
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
		fromSlot, relType, to := parseRelationFilename(name)
		if fromSlot == "" || relType == "" || to == "" {
			continue
		}
		// The FROM slot may carry a tail face ("id@face",
		// TKT-DOFYR1). An unparseable slot is skipped with a warning —
		// load tolerance, like the entity scan.
		from, fp, refErr := entity.ParseStateRef(fromSlot)
		if refErr != nil {
			slog.Warn("fsstore: skipping relation file with invalid FROM slot",
				"file", f.Name(), "error", refErr)
			continue
		}
		rm := relationMeta{From: from, Type: relType, To: to, FromFace: fp}
		key := rm.key()
		s.relations[key] = rm
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

// newestEntityFileMtime returns the newest mtime across all entity files.
// Uses stat only — no file reads.
func (s *FSStore) newestEntityFileMtime() time.Time {
	var newest time.Time
	for _, meta := range s.entities {
		key := s.layout.entityFileKey(meta.Type, stateKey(meta.ID, meta.Face))
		if info, err := s.rooted.Stat(key); err == nil {
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
		key := s.layout.relationFileKeyMeta(meta)
		if info, err := s.rooted.Stat(key); err == nil {
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

	info, err := s.rooted.Stat(s.layout.entitiesKey)
	if err != nil {
		return latest
	}
	latest = info.ModTime()

	entries, err := s.rooted.ReadDir(s.layout.entitiesKey)
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
	info, err := s.rooted.Stat(s.layout.relationsKey)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
