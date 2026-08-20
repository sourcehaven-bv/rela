package datamigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Ledger is the drift ledger: schema names whose stored data was orphaned by
// an adopted schema change, each with the time it was first seen. The GC
// engine deletes an entry's data only once the entry is older than the grace
// period; an entry whose subject reappears in the live schema is dropped
// without touching data.
//
// Entries are keyed by SCHEMA NAME, never per entity (amendment A6): the
// classifier's drift report already names what was orphaned, so no tick ever
// scans content to discover orphans. Content is read only when counting or
// deleting.
type Ledger struct {
	Entries map[string]LedgerEntry `json:"entries"`
}

// LedgerEntry is one orphaned schema name awaiting GC.
type LedgerEntry struct {
	// Kind classifies the orphan: "property" (key = "type.prop"),
	// "entity_type" (key = "type"), "relation_type" (key = "rel:type"), or
	// "relation_property" (key = "rel:type.prop"). Enum VALUES are
	// deliberately never ledgered: stale values inside a still-declared
	// property are map_values territory, not orphan cleanup.
	Kind      string    `json:"kind"`
	FirstSeen time.Time `json:"first_seen"`
}

// ledgerKV is the slice of state.KV the ledger needs.
type ledgerKV interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, data []byte) error
}

// LoadLedger reads the drift ledger; a missing or corrupt ledger is an empty
// one (it is rebuildable bookkeeping, never source data).
func LoadLedger(ctx context.Context, kv ledgerKV) (*Ledger, error) {
	data, err := kv.Get(ctx, ledgerKey)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Ledger{Entries: map[string]LedgerEntry{}}, nil
		}
		return nil, fmt.Errorf("datamigration: read drift ledger: %w", err)
	}
	var l Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		slog.Warn("datamigration.ledger_corrupt", "key", ledgerKey, "error", err)
		return &Ledger{Entries: map[string]LedgerEntry{}}, nil
	}
	if l.Entries == nil {
		l.Entries = map[string]LedgerEntry{}
	}
	return &l, nil
}

// SaveLedger persists the drift ledger.
func SaveLedger(ctx context.Context, kv ledgerKV, l *Ledger) error {
	data, err := json.Marshal(l)
	if err != nil {
		return fmt.Errorf("datamigration: marshal drift ledger: %w", err)
	}
	if err := kv.Put(ctx, ledgerKey, data); err != nil {
		return fmt.Errorf("datamigration: write drift ledger: %w", err)
	}
	return nil
}

// RecordDrift folds a shape report's deletion deltas into the ledger,
// stamping newcomers with now. Existing entries keep their original
// FirstSeen — re-observing drift must not reset the GC clock.
func (l *Ledger) RecordDrift(report metamodel.ShapeReport, now time.Time) {
	for _, d := range report.ByTier(metamodel.TierDrift) {
		var kind string
		switch d.Kind {
		case "property_removed":
			kind = "property"
			if strings.HasPrefix(d.Subject, "rel:") {
				kind = "relation_property"
			}
		case "entity_type_removed":
			kind = "entity_type"
		case "relation_type_removed":
			kind = "relation_type"
		default:
			// Everything else — new required properties, rename hints, and
			// notably REMOVED ENUM VALUES — is not GC-able drift. Enum
			// values especially: the property still exists, so "cleanup"
			// would mean erasing live-looking values; that is a map_values
			// migration decision, never a sweep's.
			continue
		}
		if _, exists := l.Entries[d.Subject]; exists {
			continue
		}
		l.Entries[d.Subject] = LedgerEntry{Kind: kind, FirstSeen: now}
	}
}

// PruneAgainst drops entries whose subject exists again in the live shape —
// the schema re-added the property/type, so the data is no longer orphaned.
// Returns the dropped keys.
func (l *Ledger) PruneAgainst(live metamodel.ShapeProjection) []string {
	var dropped []string
	for key, e := range l.Entries {
		if subjectInShape(key, e.Kind, live) {
			delete(l.Entries, key)
			dropped = append(dropped, key)
		}
	}
	return dropped
}

// Expired returns the keys of entries older than the grace period, i.e.
// eligible for GC at now.
func (l *Ledger) Expired(now time.Time, grace time.Duration) []string {
	var out []string
	for key, e := range l.Entries {
		if now.Sub(e.FirstSeen) >= grace {
			out = append(out, key)
		}
	}
	return out
}

// subjectInShape reports whether the ledger subject is (again) declared by
// the live shape.
func subjectInShape(key, kind string, live metamodel.ShapeProjection) bool {
	switch kind {
	case "entity_type":
		_, ok := live.Entities[key]
		return ok
	case "relation_type":
		_, ok := live.Relations[trimRelPrefix(key)]
		return ok
	case "property":
		owner, prop, ok := splitPropertyKey(key)
		if !ok {
			return false
		}
		es, ok := live.Entities[owner]
		if !ok {
			return false
		}
		_, ok = es.Properties[prop]
		return ok
	case "relation_property":
		owner, prop, ok := splitPropertyKey(trimRelPrefix(key))
		if !ok {
			return false
		}
		rs, ok := live.Relations[owner]
		if !ok {
			return false
		}
		_, ok = rs.Properties[prop]
		return ok
	}
	return false
}

func trimRelPrefix(s string) string {
	if rest, ok := strings.CutPrefix(s, "rel:"); ok {
		return rest
	}
	return s
}

// splitPropertyKey splits "owner.prop" at the LAST dot (entity type names
// may not contain dots today, but the last-dot rule is the safe direction:
// property names come from sortedKeys of a YAML map the operator wrote).
func splitPropertyKey(key string) (owner, prop string, ok bool) {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '.' {
			return key[:i], key[i+1:], true
		}
	}
	return "", "", false
}
