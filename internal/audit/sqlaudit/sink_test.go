package sqlaudit

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/sqlexec"
)

// fakeDB answers every transaction with fn, without running it.
type fakeDB struct {
	sqlexec.Querier
	calls atomic.Int64
	fn    func(ctx context.Context) error
}

func (f *fakeDB) Transact(ctx context.Context, _ func(q sqlexec.Querier) error) error {
	f.calls.Add(1)
	return f.fn(ctx)
}

var testTuning = sinkTuning{firstBackoff: time.Millisecond, attempts: 3, closeTimeout: 100 * time.Millisecond}

func newTestSink(t *testing.T, fn func(ctx context.Context) error) (*Sink, *fakeDB) {
	t.Helper()
	db := &fakeDB{fn: fn}
	log, err := New(db, Dialect{Name: "fake"})
	require.NoError(t, err)
	return newSink(log, testTuning), db
}

func TestSink_RetriesThenDropsAndCounts(t *testing.T) {
	var fails atomic.Int64
	s, db := newTestSink(t, func(context.Context) error {
		if fails.Add(1) <= 2 {
			return errors.New("blip")
		}
		return nil
	})
	s.Record(audit.Record{Op: audit.OpCreateEntity})
	require.Eventually(t, func() bool { return db.calls.Load() == 3 }, time.Second, time.Millisecond)
	require.NoError(t, s.Close(), "a batch that succeeds on a retry is not lost")

	s, db = newTestSink(t, func(context.Context) error { return errors.New("down") })
	s.Record(audit.Record{Op: audit.OpCreateEntity})
	s.Record(audit.Record{Op: audit.OpUpdateEntity})
	require.Eventually(t, func() bool { return db.calls.Load() >= 3 }, time.Second, time.Millisecond)
	require.ErrorContains(t, s.Close(), "2 audit records were not written")
}

func TestSink_CloseGivesUpOnAHungDatabase(t *testing.T) {
	s, _ := newTestSink(t, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	for range 3 {
		s.Record(audit.Record{Op: audit.OpCreateEntity})
	}
	start := time.Now()
	require.ErrorContains(t, s.Close(), "3 audit records were not written")
	require.Less(t, time.Since(start), 5*time.Second)
	select {
	case <-s.done:
	default:
		t.Fatal("the writer goroutine is still running")
	}
	s.Record(audit.Record{Op: audit.OpCreateEntity}) // dropped and logged
	require.NoError(t, s.Close(), "a second Close is a no-op")
}

func TestSink_QueueIsBounded(t *testing.T) {
	release := make(chan struct{})
	s, _ := newTestSink(t, func(context.Context) error {
		<-release
		return nil
	})
	s.mu.Lock()
	s.pending = make([]audit.Record, maxPending)
	s.mu.Unlock()
	s.Record(audit.Record{Op: audit.OpCreateEntity})
	s.mu.Lock()
	dropped := s.dropped
	s.mu.Unlock()
	require.Equal(t, int64(1), dropped)
	close(release)
	require.ErrorContains(t, s.Close(), "1 audit records were not written")
}

func TestNewSinkRejectsNil(t *testing.T) {
	_, err := NewSink(nil)
	require.ErrorIs(t, err, errNilLog)
}
