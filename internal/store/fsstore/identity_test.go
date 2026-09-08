package fsstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The FILENAME is an entity's identity; the frontmatter `id:`/`type:` are
// documentation of it, not a second source of truth.
//
// They used to win on load, so a hand-edited or stale `id:` produced a row
// whose identity disagreed with the index that found it: listed under the
// frontmatter id, unreachable by it (every lookup goes through the index), and
// rejected by every write with an opaque "not found". `rela analyze states`
// reported it clean (QA F-2).
func TestReadEntityFile_FilenameWinsOverFrontmatter(t *testing.T) {
	fs := storage.NewMemFS()
	rooted, err := storage.NewRootedFS(fs, "/")
	require.NoError(t, err)

	// Frontmatter claims a DIFFERENT id than the filename — including a face
	// suffix, which is not even legal in a bare id — and a different type than
	// the directory it sits in.
	const body = "---\nid: POL-001@published\ntype: other\ntitle: Mismatched\n---\nbody\n"
	require.NoError(t, fs.MkdirAll("/entities/policys", 0o755))
	require.NoError(t, fs.WriteFile("/entities/policys/POL-001.md", []byte(body), 0o644))

	s, err := New(Config{
		FS: fs, Rooted: rooted,
		EntitiesKey: "entities", RelationsKey: "relations", CacheKey: ".rela",
		Schemas: map[string]store.EntityTypeSchema{
			"policy": {Plural: "policys", PropertyOrder: []string{"title"}},
			"other":  {Plural: "others"},
		},
	})
	require.NoError(t, err)
	defer s.Close()

	e, err := s.GetEntity(context.Background(), "POL-001")
	require.NoError(t, err, "the entity must be reachable by its FILENAME id")
	require.Equal(t, "POL-001", e.ID, "frontmatter must not override the filename")
	require.Equal(t, "policy", e.Type, "the directory is authoritative for type")
	// Property values still come from the frontmatter; only IDENTITY does not.
	require.Equal(t, "Mismatched", e.Properties["title"])
}
