package fsstore

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"path"
	"slices"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mentions"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const mentionIndexFile = "mentions-index.json"

// mentionIndex maps each state key to the targets its body mentions
// (TKT-Q6AKUN). It is built on the first [FSStore.MentionSources] call rather
// than at open, so a command that never asks for backlinks reads no bodies.
// Once built, writeEntity and the watcher keep it current.
//
// Entries are never trusted to be live: a delete, a soft delete and a
// rename all take the key out of s.entities, and the read keeps only keys
// still there. That is why the delete paths need no hook, and why an unmark
// brings a family's mentions back with no work.
//
// The build reads files without holding s.mu, so a first backlink read does
// not stall every other reader and writer. A write that lands meanwhile is
// recorded in notes and wins over what the build read.
type mentionIndex struct {
	// buildMu lets one build run at a time.
	buildMu sync.Mutex
	// entries is nil until built; notes is non-nil only while a build runs.
	// Both are guarded by s.mu.
	entries map[string]mentionEntry // state key → entry
	notes   map[string]mentionEntry
}

// mentionEntry is one file's targets and the stamp of the file they were
// read from. The stamp lets a later process reuse the entry from the
// persisted cache when the file has not changed since.
type mentionEntry struct {
	ModTime time.Time `json:"mtime"`
	Size    int64     `json:"size"`
	Targets []string  `json:"targets,omitempty"`
}

// MentionSources implements [store.MentionIndex].
func (s *FSStore) MentionSources(
	_ context.Context, target string, after entity.Ref, limit int,
) ([]entity.Ref, error) {
	if limit <= 0 {
		return nil, store.ErrInvalidQuery
	}
	s.mu.RLock()
	built := s.mentions.entries != nil
	s.mu.RUnlock()
	if !built {
		buildMentionIndex(s)
	}

	s.mu.RLock()
	var out []entity.Ref
	for key, ent := range s.mentions.entries {
		meta, live := s.entities[key]
		if !live {
			continue
		}
		ref := entity.Ref{ID: meta.ID, Face: meta.Face}
		if after.ID != "" && store.CompareRefs(ref, after) <= 0 {
			continue
		}
		if _, found := slices.BinarySearch(ent.Targets, target); found {
			out = append(out, ref)
		}
	}
	s.mu.RUnlock()
	slices.SortFunc(out, store.CompareRefs)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// buildMentionIndex reads the targets of every live and marked state,
// reusing entries from the persisted cache whose file stamp still matches.
// See [mentionIndex] for why it reads outside s.mu.
func buildMentionIndex(s *FSStore) {
	m := &s.mentions
	m.buildMu.Lock()
	defer m.buildMu.Unlock()

	s.mu.Lock()
	if m.entries != nil {
		s.mu.Unlock()
		return // another caller built it while this one waited
	}
	m.notes = make(map[string]mentionEntry)
	// Marked families are indexed too: their files stay on disk, and an
	// unmark must find their entries already there.
	metas := make([]entityMeta, 0, len(s.entities))
	for _, meta := range s.entities {
		metas = append(metas, meta)
	}
	metas = append(metas, markedEntityMetas(s)...)
	s.mu.Unlock()

	cached := loadMentionCache(s)
	entries := make(map[string]mentionEntry, len(metas))
	reread := 0
	for _, meta := range metas {
		key := stateKey(meta.ID, meta.Face)
		info, err := s.rooted.Stat(s.layout.entityFileKey(meta.Type, key))
		if err != nil {
			continue
		}
		if c, ok := cached[key]; ok && c.Size == info.Size() && c.ModTime.Equal(info.ModTime()) {
			entries[key] = c
			continue
		}
		e, err := s.loadEntityMeta(meta)
		if err != nil {
			slog.Warn("fsstore: skipping unreadable entity in mention index", "key", key, "error", err)
			continue
		}
		reread++
		entries[key] = mentionEntry{
			ModTime: info.ModTime(), Size: info.Size(), Targets: mentions.Index(e.Content),
		}
	}
	if reread > 0 {
		// Saved before the notes are merged: a noted entry has no stamp to
		// match a file against.
		saveMentionCache(s, entries)
	}

	s.mu.Lock()
	maps.Copy(entries, m.notes)
	m.entries, m.notes = entries, nil
	s.mu.Unlock()
}

// noteMentions records e's targets after a write. Before the first build it
// is a no-op, since the build reads every file anyway. The entry carries no
// file stamp, so a later process rereads that file rather than trusting the
// cache. Caller must hold s.mu.
func noteMentions(s *FSStore, e *entity.Entity) {
	ent := mentionEntry{Targets: mentions.Index(e.Content)}
	key := stateKey(e.ID, e.Face)
	switch {
	case s.mentions.entries != nil:
		s.mentions.entries[key] = ent
	case s.mentions.notes != nil:
		s.mentions.notes[key] = ent
	}
}

func loadMentionCache(s *FSStore) map[string]mentionEntry {
	if s.cacheKey == "" {
		return nil
	}
	data, err := s.codec.readDataFile(path.Join(s.cacheKey, mentionIndexFile))
	if err != nil {
		return nil
	}
	var entries map[string]mentionEntry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	return entries
}

// saveMentionCache persists entries. A failure only costs the next process
// a reread, so it is logged, not returned.
func saveMentionCache(s *FSStore, entries map[string]mentionEntry) {
	if s.cacheKey == "" {
		return
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return
	}
	if err := s.rooted.WriteFile(path.Join(s.cacheKey, mentionIndexFile), data, 0o644); err != nil {
		slog.Warn("fsstore: could not persist the mention index", "error", err)
	}
}
