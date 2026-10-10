package appbuild

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
)

// The server's GC sweep drops comment threads only if it is handed the
// comment service, and a disabled service must arrive as a nil interface
// (BUG-6OZBP9).
func TestGCDeps_CommentService(t *testing.T) {
	svc, err := comments.NewService(memcomments.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := gcDeps(nil, nil, nil, nil, nil, nil, svc, nil).Comments; got != svc {
		t.Errorf("Comments = %v, want the comment service", got)
	}
	if got := gcDeps(nil, nil, nil, nil, nil, nil, nil, nil).Comments; got != nil {
		t.Errorf("Comments = %#v with commenting disabled, want a nil interface", got)
	}
}
