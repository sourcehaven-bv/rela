package pgstore_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// maxAttachmentBytes mirrors the unexported pgstore constant; kept in sync by
// the assertions below (a just-under upload succeeds, a just-over one fails).
const maxAttachmentBytes = 64 << 20

// TestAttachFileSizeCap verifies pgstore rejects an attachment larger than its
// in-database limit (defensive guard, RR for crit review) while accepting one
// at the limit. Uses a streaming reader so the test itself doesn't materialize
// >64 MiB unnecessarily for the over-limit case.
func TestAttachFileSizeCap(t *testing.T) {
	s := factory(t)
	ctx := context.Background()
	require.NoError(t, s.CreateEntity(ctx, entity.New("E-1", "ticket")))

	// At the limit: succeeds.
	atLimit := bytes.Repeat([]byte("x"), maxAttachmentBytes)
	require.NoError(t, s.AttachFile(ctx, "E-1", "blob", "ok.bin", bytes.NewReader(atLimit)))

	// Over the limit: rejected, without the store buffering the whole thing.
	over := io.MultiReader(bytes.NewReader(atLimit), strings.NewReader("y"))
	err := s.AttachFile(ctx, "E-1", "blob2", "too-big.bin", over)
	require.ErrorIs(t, err, store.ErrAttachmentTooLarge)

	// The over-limit attachment was not stored.
	_, err = s.ReadAttachment(ctx, "E-1", "blob2", "too-big.bin")
	require.Error(t, err)
}
