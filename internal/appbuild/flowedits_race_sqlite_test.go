//go:build sqlite && !postgres

package appbuild_test

import (
	"bytes"
	"runtime"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
)

// waitForWriteLockWaiter waits until the goroutine running caller is
// blocked on the sqlite store's write mutex inside Tx. Naming the caller
// keeps a background writer of the same services from counting. It fails
// when op finishes first.
func waitForWriteLockWaiter(t *testing.T, _ *appbuild.Services, op <-chan error, caller string) {
	t.Helper()
	deadline := time.Now().Add(writeLockWait)
	buf := make([]byte, 1<<22)
	for time.Now().Before(deadline) {
		failIfFinished(t, op)
		n := runtime.Stack(buf, true)
		for n == len(buf) {
			buf = make([]byte, 2*len(buf))
			n = runtime.Stack(buf, true)
		}
		for g := range bytes.SplitSeq(buf[:n], []byte("\n\n")) {
			waiting := bytes.Contains(g, []byte("sqlitestore.(*Store).Tx(")) &&
				bytes.Contains(g, []byte("sync.(*Mutex).Lock"))
			if waiting && bytes.Contains(g, []byte(caller)) {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no goroutine waited for the sqlite write lock")
}
