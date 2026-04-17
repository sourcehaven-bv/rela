package attachment

import (
	"context"
	"io"
)

// Info describes a stored attachment. Key is backend-specific (a content-
// addressable path for the CAS backend, an S3 URI for an S3 backend, etc.)
// and is the value callers typically persist on the owning entity's property.
type Info struct {
	Key          string
	EntityID     string
	Property     string
	OriginalName string
	ContentType  string
	Size         int64
}

// Manager is the top-level attachment service. Implementations cover different
// storage backends (content-addressable filesystem, S3, etc.) behind a
// minimal byte-oriented API. Concerns outside pure storage — entity-property
// updates, garbage collection policy — live at the composition layer that
// owns both the Manager and the store, not here.
type Manager interface {
	// AttachFile stores data under a backend-defined key tied to
	// (entityID, property). fileName is the caller-supplied original name and
	// is preserved in returned metadata.
	AttachFile(ctx context.Context, entityID, property, fileName string, data io.Reader) (*Info, error)

	// InfoFor looks up metadata for a previously stored key. Implementations
	// must not fall through to a zero Info on miss — return an error instead,
	// so callers can distinguish "not found" from "empty fields."
	InfoFor(ctx context.Context, key string) (*Info, error)
}
