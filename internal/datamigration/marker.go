package datamigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// State keys. Both live in state.KV, so they are per-store by construction:
// `.rela/` on the filesystem backend, the schema-scoped `state_kv` table on
// postgres (schema-per-tenant gets per-tenant migration state for free).
const (
	// markerKey holds the applied-state marker: the shape the store's
	// content currently conforms to.
	markerKey = "migration/state.json"
	// ledgerKey holds the drift ledger: schema names whose data is
	// orphaned, with first-seen timestamps, awaiting GC.
	ledgerKey = "migration/drift.json"
)

// Marker records the schema shape a store's content conforms to. The FULL
// projection is stored, not just the hash: `rela migrate gen` diffs it
// against the live schema, and the chain resolver compares it against a
// migration's embedded from-projection — neither works from a hash alone.
type Marker struct {
	ShapeHash string `json:"shape_hash"`
	// Projection is the ShapeProjection JSON ([metamodel.ShapeProjection.JSON]).
	Projection json.RawMessage `json:"projection"`
	// Applied lists the names of migration files already applied, so a
	// re-run of an already-applied migration is skipped.
	Applied   []string  `json:"applied,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ShapeProjection parses the marker's stored projection.
func (m *Marker) ShapeProjection() (metamodel.ShapeProjection, error) {
	return metamodel.ShapeProjectionFromJSON(m.Projection)
}

// LoadMarker reads the applied-state marker. A missing key returns (nil, nil)
// — the caller bootstraps. A corrupt marker is treated as absent WITH a
// logged warning rather than an error: the gate must never wedge startup on
// unreadable state, and re-bootstrapping is safe (it adopts the live shape).
func LoadMarker(ctx context.Context, kv state.KV) (*Marker, error) {
	data, err := kv.Get(ctx, markerKey)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil //nolint:nilnil // absent marker = un-bootstrapped store, a normal state the gate handles
		}
		return nil, fmt.Errorf("datamigration: read marker: %w", err)
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		slog.Warn("datamigration.marker_corrupt", "key", markerKey, "error", err)
		return nil, nil //nolint:nilnil // corrupt marker is treated as absent (re-bootstrap), never a startup failure
	}
	if m.ShapeHash == "" || len(m.Projection) == 0 {
		slog.Warn("datamigration.marker_incomplete", "key", markerKey)
		return nil, nil //nolint:nilnil // incomplete marker is treated as absent (re-bootstrap)
	}
	return &m, nil
}

// SaveMarker writes the applied-state marker. Callers serialize writes via
// the migration lock where one exists (state.KV has no compare-and-swap);
// adoption races are benign because racers write identical content.
func SaveMarker(ctx context.Context, kv state.KV, m *Marker) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("datamigration: marshal marker: %w", err)
	}
	if err := kv.Put(ctx, markerKey, data); err != nil {
		return fmt.Errorf("datamigration: write marker: %w", err)
	}
	return nil
}

// NewMarker builds a marker for the given projection at the current time.
func NewMarker(proj metamodel.ShapeProjection, applied []string, now time.Time) (*Marker, error) {
	projJSON, err := proj.JSON()
	if err != nil {
		return nil, fmt.Errorf("datamigration: marshal projection: %w", err)
	}
	return &Marker{
		ShapeHash:  proj.Hash(),
		Projection: projJSON,
		Applied:    applied,
		UpdatedAt:  now,
	}, nil
}
