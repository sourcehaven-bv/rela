//go:build !postgres && !memorybackend

package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func TestBackfillBackend_NilSafe(t *testing.T) {
	assert.NoError(t, backfillMCPBackend(context.Background(), nil, memstore.New()))
}
