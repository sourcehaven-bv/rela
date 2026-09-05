package memcomments_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/commentstest"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
)

func TestConformance(t *testing.T) {
	commentstest.RunAll(t, func(*testing.T) comments.Store {
		return memcomments.New()
	})
}
