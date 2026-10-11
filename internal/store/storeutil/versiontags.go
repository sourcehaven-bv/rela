package storeutil

import (
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LineageRow is one row of a face's fenced lineage walk, as a versioning
// backend reads it: the metadata a history list shows, plus the row's vseq,
// the entity id it was captured under, and the exclusive upper vseq fence of
// that id's segment in the lineage (0 for the head segment, which is
// unbounded).
//
// Shared by pgstore and sqlitestore so the version-tag lifecycle rule
// (TKT-VO6VG9) is written once rather than once per SQL dialect.
type LineageRow struct {
	Meta     store.VersionMeta
	Vseq     int64
	EntityID string
	Hi       int64
}

// TagRow is one version_tags row: the tagged vseq and the tag name.
type TagRow struct {
	Vseq int64
	Name string
}

// FinishLineage sorts rows by vseq, drops duplicates (a rename diamond can
// match one row twice) and assigns the 1-based ordinals ListVersions shows.
func FinishLineage(rows []LineageRow) []LineageRow {
	slices.SortStableFunc(rows, func(a, b LineageRow) int {
		switch {
		case a.Vseq < b.Vseq:
			return -1
		case a.Vseq > b.Vseq:
			return 1
		}
		return 0
	})
	rows = slices.CompactFunc(rows, func(a, b LineageRow) bool { return a.Vseq == b.Vseq })
	for i := range rows {
		rows[i].Meta.Version = i + 1
	}
	return rows
}

// CurrentLifecycle returns the vseqs of the rows in the face's current
// lifecycle: those no later delete has ended.
//
// A delete ends a row when it is newer and either belongs to the same
// segment, or lies at or past the end of the row's segment, which places it
// in a later part of the chain. A delete in a later segment that predates the
// rename into it belongs to an earlier, unrelated occupant of that id, so it
// does not end the chain that was renamed in. That is what lets a rename onto
// a previously deleted id keep its own tags while the old occupant's tags stay
// behind.
func CurrentLifecycle(rows []LineageRow) map[int64]bool {
	var deletes []LineageRow
	for _, r := range rows {
		if r.Meta.Op == store.VersionOpDelete {
			deletes = append(deletes, r)
		}
	}
	out := make(map[int64]bool, len(rows))
	for _, r := range rows {
		ended := false
		for _, d := range deletes {
			if d.Vseq > r.Vseq && (d.EntityID == r.EntityID || (r.Hi != 0 && d.Vseq >= r.Hi)) {
				ended = true
				break
			}
		}
		if !ended {
			out[r.Vseq] = true
		}
	}
	return out
}

// LineageVseqs returns every vseq of rows, for the all-lifecycle tag delete
// a move or untag performs.
func LineageVseqs(rows []LineageRow) []int64 {
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = r.Vseq
	}
	return out
}

// ApplyTags fills Meta.Tags of each current-lifecycle row from tags, sorted
// by name. Tags on rows outside the current lifecycle are not shown.
func ApplyTags(rows []LineageRow, tags []TagRow) {
	if len(tags) == 0 {
		return
	}
	current := CurrentLifecycle(rows)
	byVseq := make(map[int64][]string, len(tags))
	for _, t := range tags {
		if current[t.Vseq] {
			byVseq[t.Vseq] = append(byVseq[t.Vseq], t.Name)
		}
	}
	for i := range rows {
		if names := byVseq[rows[i].Vseq]; len(names) > 0 {
			slices.Sort(names)
			rows[i].Meta.Tags = names
		}
	}
}

// Metas returns the rows' metadata in order.
func Metas(rows []LineageRow) []store.VersionMeta {
	out := make([]store.VersionMeta, len(rows))
	for i, r := range rows {
		out[i] = r.Meta
	}
	return out
}

// TaggedRow returns the newest current-lifecycle row that tags names name.
// ok is false when there is none.
func TaggedRow(rows []LineageRow, tags []TagRow, name string) (row LineageRow, ok bool) {
	current := CurrentLifecycle(rows)
	var best int64
	for _, t := range tags {
		if t.Name == name && current[t.Vseq] && t.Vseq > best {
			best = t.Vseq
		}
	}
	if best == 0 {
		return LineageRow{}, false
	}
	return RowByVseq(rows, best)
}

// RowByVseq returns the row with vseq v.
func RowByVseq(rows []LineageRow, v int64) (LineageRow, bool) {
	for _, r := range rows {
		if r.Vseq == v {
			return r, true
		}
	}
	return LineageRow{}, false
}

// Taggable reports whether a version row may carry a tag: it must hold
// content (not a delete or purge marker) and be in the current lifecycle.
func Taggable(r LineageRow, current map[int64]bool) bool {
	if r.Meta.Op == store.VersionOpDelete || r.Meta.Op == store.VersionOpPurge {
		return false
	}
	return current[r.Vseq]
}
