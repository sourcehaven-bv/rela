package dataentry

import (
	"context"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/changefeed"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

type emptyFeed struct{}

func (emptyFeed) Wake() <-chan struct{}  { return nil }
func (emptyFeed) Take() changefeed.Batch { return changefeed.Batch{} }
func (emptyFeed) Token(uint64) string    { return "" }

// TestSSE_NewDayRegates pins that a stream re-derives its read gate on a new
// UTC day: a conditional grant (TKT-TKSCSBT) judged at the stream's opening
// time must not keep nudging after it expires.
func TestSSE_NewDayRegates(t *testing.T) {
	prev := nowFunc
	t.Cleanup(func() { nowFunc = prev })
	opened := time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return opened.Add(30 * time.Minute) }

	st := &sseStream{
		feed: emptyFeed{}, verdicts: map[string]bool{}, pending: map[string]struct{}{},
		gateDay: readDay(opened),
	}
	if st.take(context.Background()) || st.regate {
		t.Fatal("the same UTC day must not re-derive the gate")
	}
	nowFunc = func() time.Time { return opened.Add(2 * time.Hour) }
	if !st.take(context.Background()) || !st.regate {
		t.Fatal("a new UTC day must re-derive the gate at the next flush")
	}
}

// TestSSE_FailedNewDayRegateWithholds pins that a stream whose gate cannot be
// re-derived on a new UTC day sends nothing: the prior gate was judged on an
// earlier day and may still count an expired grant. The context carries no
// principal, so opening a request fails.
func TestSSE_FailedNewDayRegateWithholds(t *testing.T) {
	prev := nowFunc
	t.Cleanup(func() { nowFunc = prev })
	nowFunc = func() time.Time { return time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC) }

	ms := memstore.New()
	d, err := acl.NewDeclarative(&acl.Policy{}, aclstore.MustNewGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	st := &sseStream{
		app: &App{acl: d}, feed: emptyFeed{}, verdicts: map[string]bool{},
		pending: map[string]struct{}{"ticket": {}}, regate: true, gateDay: "2026-12-31", cursor: 7,
	}
	// conn is nil: a frame would panic, so reaching the end proves nothing was sent.
	if err := st.flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if !st.regate || st.gateDay != "2026-12-31" {
		t.Fatalf("a failed re-derive must stay pending: regate=%v gateDay=%s", st.regate, st.gateDay)
	}
	if len(st.pending) != 0 || st.flushed != 7 {
		t.Fatalf("the withheld notices must be dropped: pending=%v flushed=%d", st.pending, st.flushed)
	}
}
