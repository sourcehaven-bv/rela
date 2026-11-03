package realtime

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Change names an entity the store reported as changed. The trigger engine
// reads the entity again, so a change carries only which one it was.
type Change struct {
	EntityType string
	ID         string
}

// FaceMode is which faces of an entity a rule reads.
type FaceMode int

// Face modes. A trigger reads the face its channel's readers see.
const (
	// FacePrime reads the default world's prime face, for a channel
	// anchored to the whole family.
	FacePrime FaceMode = iota + 1
	// FaceEach reads every face on its own, for a channel anchored to a
	// face.
	FaceEach
)

// EntityRow is one face of an entity as the trigger engine reads it.
type EntityRow struct {
	ID    string
	Face  entity.Face
	Props map[string]any
	// Inaccessible lists the properties whose stored value cannot be read,
	// such as an undecryptable file's. A rule that reads one leaves what it
	// has seen of the entity as it was.
	Inaccessible []string
}

// EntitySource reads entity headers for the trigger engine, with no ACL:
// the engine acts for the system, and a rule may read only properties no
// `visible:` grant restricts (realtimecfg refuses the others at load).
type EntitySource interface {
	// Rows reads the given ids of entityType in one batch. An id that does
	// not exist, or has no face m selects, is absent from the result.
	Rows(ctx context.Context, entityType string, ids []string, m FaceMode) ([]EntityRow, error)
	// Scan reads every entity of entityType, a page at a time.
	Scan(ctx context.Context, entityType string, m FaceMode, page func([]EntityRow) error) error
}

// KV is the key-value state the trigger engine keeps what it has seen in.
// On a backend that several processes share, every process sees the same
// values.
type KV interface {
	// Get reads the value at key. A key with no value is an error for
	// which errors.Is with fs.ErrNotExist, or os.IsNotExist, holds.
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, data []byte) error
	// Delete removes the value at key; a missing key is no error.
	Delete(ctx context.Context, key string) error
}
