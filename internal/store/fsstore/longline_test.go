package fsstore_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestLongLine_WriteThenReadBack is the store-level regression for the
// 64 KB frontmatter-split cap: an entity whose body contains a single
// line larger than bufio.MaxScanTokenSize (a base64 data: URI, a pasted
// log, a minified blob) must round-trip. The old bufio.Scanner-based
// split wrote the file fine but returned bufio.ErrTooLong on read,
// making the entity writable-but-unreadable.
func TestLongLine_WriteThenReadBack(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	const bigLineSize = 256 * 1024 // 4x the scanner's 64 KB cap
	bigLine := strings.Repeat("x", bigLineSize)

	s1 := openStore(t, fs)
	e := entity.New("REQ-1", "requirement")
	e.Properties["title"] = "Has a very long body line"
	e.Content = "Before\n" + bigLine + "\nAfter"
	require.NoError(t, s1.CreateEntity(ctx, e))

	// Read back from the same store instance...
	got, err := s1.GetEntity(ctx, "REQ-1")
	require.NoError(t, err, "entity with a >64KB line must be readable")
	assert.Contains(t, got.Content, bigLine, "the long line must survive the round-trip")
	require.NoError(t, s1.Close())

	// ...and after reopening from disk (the cold-read path).
	s2 := openStore(t, fs)
	defer s2.Close()
	got2, err := s2.GetEntity(ctx, "REQ-1")
	require.NoError(t, err, "entity with a >64KB line must be readable after reopen")
	assert.Contains(t, got2.Content, bigLine)
}
