package reconclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/reconwire"
)

func TestDiff_DeadlineIsBudget(t *testing.T) {
	s := newSet(tickets(3, "x")...)
	opt := Options{Deadline: time.Millisecond, Sleep: func(ctx context.Context, _ time.Duration) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	_, err := diff(t, &fakeLocal{s: s}, &fakeRemote{s: s, refuse: 5}, Scope{Entities: []string{"ticket"}}, opt)
	if !errors.Is(err, ErrBudget) || !strings.Contains(err.Error(), "took longer than") {
		t.Errorf("err = %v, want a budget error naming the deadline", err)
	}
}

// TestDiff_LocalTooLargeMultiTypeListsOnce: a multi-type span the local side
// will not fingerprint is split by type before anything is listed, so every
// local row is listed once.
func TestDiff_LocalTooLargeMultiTypeListsOnce(t *testing.T) {
	els := tickets(2500, "x")
	for i := range 2500 {
		els = append(els, ent(fmt.Sprintf("feature/FEAT-%05d", i), "x"))
	}
	local := &fakeLocal{s: newSet(els...), tooLarge: 2000}
	remote := &fakeRemote{s: newSet(append(els, ent("ticket/TKT-99999", "r"))...)}
	res, err := diff(t, local, remote, Scope{Entities: []string{"feature", "ticket"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := addrs(res.OnlyRemote); len(got) != 1 || got[0] != "ticket/TKT-99999" {
		t.Errorf("only remote %v", got)
	}
	if res.Stats.LocalElements != 5000 {
		t.Errorf("listed %d local elements, want 5000", res.Stats.LocalElements)
	}
}

// TestDiff_CountsMustAddUp: a remote whose parts do not add up to the whole
// is caught, so it cannot keep the client splitting on local addresses.
func TestDiff_CountsMustAddUp(t *testing.T) {
	local := &fakeLocal{s: newSet(tickets(1000, "l")...)}
	lie := func(req reconwire.Request, resp *reconwire.Response) {
		for i, rr := range req.Ranges {
			if rr.Op == "fingerprint" {
				n := uint64(5000)
				resp.Ranges[i].Count = &n
				resp.Ranges[i].Fingerprint = strings.Repeat("1", 32)
			}
		}
	}
	remote := &fakeRemote{mutate: lie}
	_, err := diff(t, local, remote, Scope{Entities: []string{"ticket"}}, Options{})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("err = %v, want ErrProtocol", err)
	}
	if len(remote.requests) != 2 {
		t.Errorf("%d requests, want the lie caught on the second", len(remote.requests))
	}
}

// TestDiff_SmallLocalSpanIsNotSplit: a span with few local elements is
// listed, so none of its local addresses go out as range bounds.
func TestDiff_SmallLocalSpanIsNotSplit(t *testing.T) {
	remote := &fakeRemote{s: newSet(tickets(5000, "r")...)}
	_, err := diff(t, &fakeLocal{s: newSet(tickets(200, "l")...)}, remote, Scope{Entities: []string{"ticket"}},
		Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range remote.requests {
		for _, rr := range req.Ranges {
			if rr.Lo != "" || rr.Hi != nil {
				t.Fatalf("a range bound was sent: lo %q", rr.Lo)
			}
		}
	}
}

// unnamedRange drops the range index from a remote's refusal.
type unnamedRange struct{ *fakeRemote }

func (u unnamedRange) Reconcile(ctx context.Context, req reconwire.Request) (reconwire.Response, error) {
	resp, err := u.fakeRemote.Reconcile(ctx, req)
	var re *RangeError
	if errors.As(err, &re) {
		re.Index = -1
	}
	return resp, err
}

func TestDiff_RefusalWithoutRangeIndex(t *testing.T) {
	s := newSet(ent("ticket/TKT-1", "t"), ent("feature/FEAT-1", "f"), rel("TKT-1--blocks--TKT-1", "b"))
	scope := Scope{Entities: []string{"feature", "ticket"}, Relations: []string{"blocks"}}
	res, err := diff(t, &fakeLocal{s: s}, unnamedRange{&fakeRemote{s: s, gated: []string{"blocks"}}}, scope,
		Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Type != "blocks" {
		t.Errorf("skipped %+v", res.Skipped)
	}
}

func TestSoleRelation(t *testing.T) {
	e := &task{sp: &span{kind: KindEntity}}
	r := &task{sp: &span{kind: KindRelation}}
	for _, tc := range []struct {
		batch []*task
		want  int
	}{
		{[]*task{e, r}, 1}, {[]*task{e}, -1}, {[]*task{r, e, r}, -1},
	} {
		if got := soleRelation(tc.batch); got != tc.want {
			t.Errorf("soleRelation = %d, want %d", got, tc.want)
		}
	}
	if _, err := (&run{}).remoteGated([]*task{r, r}, &RangeError{Index: -1}); !errors.Is(err, ErrProtocol) {
		t.Errorf("err = %v, want ErrProtocol", err)
	}
}

func TestDiff_RemoteBytesBudget(t *testing.T) {
	remote := &fakeRemote{s: newSet(tickets(100, "r")...)}
	_, err := diff(t, &fakeLocal{}, remote, Scope{Entities: []string{"ticket"}}, Options{MaxRemoteBytes: 500})
	if !errors.Is(err, ErrBudget) {
		t.Errorf("err = %v, want ErrBudget", err)
	}
}

func TestDiff_LongCursorIsRefused(t *testing.T) {
	long := "ticket/" + strings.Repeat("a", 4089)
	mutate := func(req reconwire.Request, resp *reconwire.Response) {
		for i, rr := range req.Ranges {
			if rr.Op == "list" {
				resp.Ranges[i] = reconwire.Result{Status: reconwire.StatusOK, Last: long, More: true}
			}
		}
	}
	_, err := diff(t, &fakeLocal{}, &fakeRemote{s: newSet(tickets(3, "r")...), mutate: mutate},
		Scope{Entities: []string{"ticket"}}, Options{})
	if !errors.Is(err, ErrProtocol) {
		t.Errorf("err = %v, want ErrProtocol", err)
	}
}

// TestDiff_TooLargeAlwaysLeaksFewBounds: a remote that answers too_large to
// every fingerprint gets one split of the large span, at bucketRows local
// elements per bound, and listings after that. It never sees finer bounds.
func TestDiff_TooLargeAlwaysLeaksFewBounds(t *testing.T) {
	const n = 4000
	local := &fakeLocal{s: newSet(tickets(n, "l")...)}
	remote := &fakeRemote{s: newSet(tickets(n, "r")...), tooLarge: -1}
	res, err := diff(t, local, remote, Scope{Entities: []string{"ticket"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Differing) != n {
		t.Errorf("%d differing, want %d", len(res.Differing), n)
	}
	bounds := map[string]bool{}
	for _, req := range remote.requests {
		for _, rr := range req.Ranges {
			if rr.Lo != "" {
				bounds[rr.Lo] = true
			}
			if rr.Hi != nil {
				bounds[*rr.Hi] = true
			}
		}
	}
	if want := (n + bucketRows - 1) / bucketRows; len(bounds) > want-1 {
		t.Errorf("the remote saw %d local bounds, want at most %d", len(bounds), want-1)
	}
}

// TestDiff_TooLargeSmallSpanIsListed: too_large for a span with few local
// elements lists it rather than splitting on local addresses.
func TestDiff_TooLargeSmallSpanIsListed(t *testing.T) {
	local := &fakeLocal{s: newSet(tickets(bucketRows, "l")...)}
	remote := &fakeRemote{s: newSet(tickets(bucketRows, "r")...), tooLarge: -1}
	if _, err := diff(t, local, remote, Scope{Entities: []string{"ticket"}}, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, req := range remote.requests {
		for _, rr := range req.Ranges {
			if rr.Lo != "" || rr.Hi != nil {
				t.Fatalf("a range with local bounds was sent: %+v", rr)
			}
		}
	}
}
