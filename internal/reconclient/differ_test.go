package reconclient

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/reconwire"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

var testScope = Scope{Entities: []string{"feature", "ticket"}, Relations: []string{"blocks", "implements"}}

func noSleep(context.Context, time.Duration) error { return nil }

func diff(t *testing.T, local Local, remote Remote, scope Scope, opt Options) (Result, error) {
	t.Helper()
	if opt.Sleep == nil {
		opt.Sleep = noSleep
	}
	d, err := New(local, remote, opt)
	if err != nil {
		t.Fatal(err)
	}
	return d.Diff(t.Context(), scope)
}

func mustDiff(t *testing.T, local *fakeLocal, remote *fakeRemote) Result {
	t.Helper()
	res, err := diff(t, local, remote, testScope, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func addrs(els []Element) []string {
	out := make([]string, len(els))
	for i, e := range els {
		out[i] = e.Address
	}
	return out
}

func changed(cs []Change) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Address
	}
	return out
}

// tickets returns n ticket elements, ticket/TKT-00000 up, with content c.
func tickets(n int, c string) []Element {
	out := make([]Element, n)
	for i := range n {
		out[i] = ent(fmt.Sprintf("ticket/TKT-%05d", i), c)
	}
	return out
}

func TestNew_RejectsNil(t *testing.T) {
	if _, err := New(nil, &fakeRemote{}, Options{}); err == nil {
		t.Error("nil local accepted")
	}
	if _, err := New(&fakeLocal{}, nil, Options{}); err == nil {
		t.Error("nil remote accepted")
	}
}

func TestServerMaxListRows_MatchesVisibility(t *testing.T) {
	if serverMaxListRows != visibility.DefaultElementMaxRows {
		t.Errorf("serverMaxListRows %d != visibility.DefaultElementMaxRows %d",
			serverMaxListRows, visibility.DefaultElementMaxRows)
	}
}

func TestDiff_Sets(t *testing.T) {
	base := []Element{
		ent("feature/FEAT-1", "f1"), ent("ticket/TKT-1", "t1"), ent("ticket/TKT-2", "t2"),
		rel("TKT-1--implements--FEAT-1", "r1"),
	}
	cases := []struct {
		name              string
		local, remote     []Element
		onlyL, onlyR, chg []string
	}{
		{name: "identical", local: base, remote: base},
		{name: "both empty"},
		{
			name: "remote empty", local: base,
			onlyL: []string{"feature/FEAT-1", "ticket/TKT-1", "ticket/TKT-2", "TKT-1--implements--FEAT-1"},
		},
		{
			name: "local empty", remote: base,
			onlyR: []string{"feature/FEAT-1", "ticket/TKT-1", "ticket/TKT-2", "TKT-1--implements--FEAT-1"},
		},
		{
			name:  "overlapping",
			local: base,
			remote: []Element{
				ent("feature/FEAT-1", "f1"), ent("ticket/TKT-2", "t2"), ent("ticket/TKT-3", "t3"),
				rel("TKT-1--implements--FEAT-1", "r1"), rel("TKT-2--blocks--TKT-3", "b"),
			},
			onlyL: []string{"ticket/TKT-1"},
			onlyR: []string{"ticket/TKT-3", "TKT-2--blocks--TKT-3"},
		},
		{
			name:  "differing hash",
			local: base,
			remote: []Element{
				ent("feature/FEAT-1", "f1"), ent("ticket/TKT-1", "changed"), ent("ticket/TKT-2", "t2"),
				rel("TKT-1--implements--FEAT-1", "r1 changed"),
			},
			chg: []string{"ticket/TKT-1", "TKT-1--implements--FEAT-1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			remote := &fakeRemote{s: newSet(tc.remote...)}
			res := mustDiff(t, &fakeLocal{s: newSet(tc.local...)}, remote)
			if got := addrs(res.OnlyLocal); !slices.Equal(got, tc.onlyL) && len(got)+len(tc.onlyL) > 0 {
				t.Errorf("only local = %v, want %v", got, tc.onlyL)
			}
			if got := addrs(res.OnlyRemote); !slices.Equal(got, tc.onlyR) && len(got)+len(tc.onlyR) > 0 {
				t.Errorf("only remote = %v, want %v", got, tc.onlyR)
			}
			if got := changed(res.Differing); !slices.Equal(got, tc.chg) && len(got)+len(tc.chg) > 0 {
				t.Errorf("differing = %v, want %v", got, tc.chg)
			}
			if res.Same() != (len(tc.onlyL)+len(tc.onlyR)+len(tc.chg) == 0) {
				t.Errorf("Same() = %v", res.Same())
			}
			if res.Stats.RoundTrips != len(remote.requests) {
				t.Errorf("round trips %d, requests %d", res.Stats.RoundTrips, len(remote.requests))
			}
		})
	}
}

func TestDiff_IdenticalCostsOneRoundTrip(t *testing.T) {
	s := newSet(append(tickets(5000, "x"), ent("feature/FEAT-1", "f"))...)
	remote := &fakeRemote{s: s}
	res := mustDiff(t, &fakeLocal{s: s}, remote)
	if !res.Same() || res.Stats.RoundTrips != 1 {
		t.Errorf("same=%v after %d round trips, want true after 1", res.Same(), res.Stats.RoundTrips)
	}
	if res.Stats.RemoteElements != 0 || res.Stats.LocalElements != 0 {
		t.Errorf("listed %d remote and %d local elements for identical sets", res.Stats.RemoteElements,
			res.Stats.LocalElements)
	}
}

// TestDiff_Budget pins the round trips and the rows moved for a large type
// with few differences, for a reader the remote can fingerprint in one go
// and for one whose ranges are too_large over 2000 rows (hidden fields).
func TestDiff_Budget(t *testing.T) {
	const n = 10000
	cases := []struct {
		name       string
		diffs      int
		tooLarge   int
		maxTrips   int
		maxListed  int
		wantChange int
	}{
		{"no difference", 0, 0, 1, 0, 0},
		{"one difference", 1, 0, 4, 100, 1},
		{"five differences", 5, 0, 6, 300, 5},
		{"one difference, too_large over 2000", 1, 2000, 4, 100, 1},
		// After too_large the first split is into buckets of bucketRows, not
		// fanout, so fewer local bounds go out and more rows are listed.
		{"five differences, too_large over 2000", 5, 2000, 6, 500, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			local := tickets(n, "same")
			remote := slices.Clone(local)
			for i := range tc.diffs {
				j := (i*1777 + 13) % n
				remote[j] = ent(remote[j].Address, "changed")
			}
			fr := &fakeRemote{s: newSet(remote...), tooLarge: tc.tooLarge}
			res, err := diff(t, &fakeLocal{s: newSet(local...), tooLarge: tc.tooLarge}, fr,
				Scope{Entities: []string{"ticket"}}, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Differing) != tc.wantChange || len(res.OnlyLocal)+len(res.OnlyRemote) != 0 {
				t.Fatalf("got %d changes, %d only local, %d only remote", len(res.Differing),
					len(res.OnlyLocal), len(res.OnlyRemote))
			}
			if res.Stats.RoundTrips > tc.maxTrips {
				t.Errorf("%d round trips, budget %d", res.Stats.RoundTrips, tc.maxTrips)
			}
			if res.Stats.RemoteElements > tc.maxListed {
				t.Errorf("%d remote elements listed, budget %d", res.Stats.RemoteElements, tc.maxListed)
			}
			t.Logf("%d round trips, %d fingerprint ranges, %d remote and %d local elements listed",
				res.Stats.RoundTrips, fr.fpRanges(), res.Stats.RemoteElements, res.Stats.LocalElements)
		})
	}
}

func TestDiff_RemoteHeavyRangeIsListed(t *testing.T) {
	remote := &fakeRemote{s: newSet(tickets(3000, "r")...)}
	res, err := diff(t, &fakeLocal{s: newSet(ent("ticket/TKT-00010", "r"))}, remote,
		Scope{Entities: []string{"ticket"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.OnlyRemote) != 2999 || len(res.OnlyLocal) != 0 || len(res.Differing) != 0 {
		t.Errorf("got %d only remote, %d only local, %d differing", len(res.OnlyRemote), len(res.OnlyLocal),
			len(res.Differing))
	}
}

func TestDiff_Paging(t *testing.T) {
	remote := &fakeRemote{s: newSet(tickets(4500, "r")...)}
	res, err := diff(t, &fakeLocal{}, remote, Scope{Entities: []string{"ticket"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.OnlyRemote) != 4500 {
		t.Fatalf("only remote %d, want 4500", len(res.OnlyRemote))
	}
	afters := 0
	for _, req := range remote.requests {
		for _, rr := range req.Ranges {
			if rr.After != nil {
				afters++
			}
		}
	}
	if afters != 2 {
		t.Errorf("%d follow-up pages, want 2", afters)
	}
}

func TestDiff_LocalTooLargeIsListed(t *testing.T) {
	s := newSet(tickets(3000, "x")...)
	local := &fakeLocal{s: s, tooLarge: 2000}
	res, err := diff(t, local, &fakeRemote{s: s}, Scope{Entities: []string{"ticket"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Same() || res.Stats.RoundTrips != 1 || res.Stats.LocalElements != 3000 {
		t.Errorf("same=%v trips=%d local=%d", res.Same(), res.Stats.RoundTrips, res.Stats.LocalElements)
	}
}

func TestDiff_NotRun(t *testing.T) {
	s := newSet(tickets(10, "x")...)
	t.Run("asked again", func(t *testing.T) {
		remote := &fakeRemote{s: s, notRun: 1}
		res, err := diff(t, &fakeLocal{s: s}, remote, Scope{Entities: []string{"ticket"}}, Options{})
		if err != nil || !res.Same() || res.Stats.RoundTrips != 2 {
			t.Errorf("err=%v same=%v trips=%d", err, res.Same(), res.Stats.RoundTrips)
		}
	})
	t.Run("always not_run fails", func(t *testing.T) {
		remote := &fakeRemote{s: s, notRun: 1 << 30}
		_, err := diff(t, &fakeLocal{s: s}, remote, Scope{Entities: []string{"ticket"}}, Options{})
		if !errors.Is(err, ErrBudget) {
			t.Fatalf("err = %v, want ErrBudget", err)
		}
		if len(remote.requests) > 40 {
			t.Errorf("%d requests before giving up", len(remote.requests))
		}
	})
}

func TestDiff_RefusalsWait(t *testing.T) {
	s := newSet(tickets(3, "x")...)
	var waits []time.Duration
	opt := Options{MaxRetryWait: 2 * time.Second, Sleep: func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}}
	remote := &fakeRemote{s: s, refuse: 2}
	res, err := diff(t, &fakeLocal{s: s}, remote, Scope{Entities: []string{"ticket"}}, opt)
	if err != nil || !res.Same() {
		t.Fatalf("err=%v same=%v", err, res.Same())
	}
	if !slices.Equal(waits, []time.Duration{2 * time.Second, 2 * time.Second}) {
		t.Errorf("waits %v, want two of 2s (Retry-After 3s capped)", waits)
	}
	if res.Stats.RoundTrips != 3 {
		t.Errorf("round trips %d, want 3", res.Stats.RoundTrips)
	}

	remote = &fakeRemote{s: s, refuse: 100}
	if _, err := diff(t, &fakeLocal{s: s}, remote, Scope{Entities: []string{"ticket"}}, Options{MaxRefusals: 3}); !errors.Is(err, ErrBudget) {
		t.Errorf("err = %v, want ErrBudget", err)
	}
	if len(remote.requests) != 3 {
		t.Errorf("%d requests, want 3", len(remote.requests))
	}
}

func TestDiff_MetaGated(t *testing.T) {
	s := newSet(ent("ticket/TKT-1", "t"), ent("feature/FEAT-1", "f"),
		rel("TKT-1--implements--FEAT-1", "i"), rel("TKT-1--blocks--TKT-1", "b"))
	for _, side := range []string{"local", "remote"} {
		t.Run(side, func(t *testing.T) {
			local, remote := &fakeLocal{s: s}, &fakeRemote{s: s}
			if side == "local" {
				local.gated = []string{"blocks"}
				local.s = newSet(ent("ticket/TKT-1", "t"), ent("feature/FEAT-1", "f"),
					rel("TKT-1--implements--FEAT-1", "i"))
			} else {
				remote.gated = []string{"blocks"}
			}
			res, err := diff(t, local, remote, testScope, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !res.Same() {
				t.Errorf("differences: %+v", res)
			}
			want := []Skipped{{Kind: KindRelation, Type: "blocks", Remote: side == "remote"}}
			if len(res.Skipped) != 1 || res.Skipped[0].Type != want[0].Type || res.Skipped[0].Remote != want[0].Remote {
				t.Errorf("skipped %+v, want %+v", res.Skipped, want)
			}
		})
	}
}

func TestDiff_InitialSpansStayUnderTypeNameCap(t *testing.T) {
	var ents []string
	for i := range 300 {
		ents = append(ents, fmt.Sprintf("t%03d", i))
	}
	remote := &fakeRemote{}
	if _, err := diff(t, &fakeLocal{}, remote, Scope{Entities: ents}, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := diff(t, &fakeLocal{}, remote, Scope{Entities: ents, Relations: []string{"r"}}, Options{}); err == nil {
		t.Error("relations with 300 entity types in scope accepted")
	}
}

func TestDiff_ScopeErrors(t *testing.T) {
	for _, sc := range []Scope{
		{Relations: []string{"blocks"}},
		{Entities: []string{"ticket", "ticket"}},
		{Entities: []string{""}},
	} {
		if _, err := diff(t, &fakeLocal{}, &fakeRemote{}, sc, Options{}); err == nil {
			t.Errorf("scope %+v accepted", sc)
		}
	}
	remote := &fakeRemote{}
	res, err := diff(t, &fakeLocal{}, remote, Scope{}, Options{})
	if err != nil || !res.Same() || len(remote.requests) != 0 {
		t.Errorf("empty scope: err=%v, %d requests", err, len(remote.requests))
	}
}

func TestDiff_Budgets(t *testing.T) {
	big := newSet(tickets(3000, "x")...)
	if _, err := diff(t, &fakeLocal{}, &fakeRemote{s: big}, Scope{Entities: []string{"ticket"}},
		Options{MaxRemoteElements: 1500}); !errors.Is(err, ErrBudget) {
		t.Errorf("remote cap: %v", err)
	}
	if _, err := diff(t, &fakeLocal{s: big, tooLarge: 2000}, &fakeRemote{}, Scope{Entities: []string{"ticket"}},
		Options{MaxLocalElements: 1500}); !errors.Is(err, ErrBudget) {
		t.Errorf("local cap: %v", err)
	}
	if _, err := diff(t, &fakeLocal{}, &fakeRemote{s: big}, Scope{Entities: []string{"ticket"}},
		Options{MaxRoundTrips: 2}); !errors.Is(err, ErrBudget) {
		t.Errorf("round trip cap: %v", err)
	}
}

func TestDiff_Deadline(t *testing.T) {
	s := newSet(tickets(3, "x")...)
	opt := Options{Deadline: time.Millisecond, Sleep: func(ctx context.Context, _ time.Duration) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	_, err := diff(t, &fakeLocal{s: s}, &fakeRemote{s: s, refuse: 5}, Scope{Entities: []string{"ticket"}}, opt)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want deadline", err)
	}
}

// TestDiff_MalformedAnswers: each broken answer fails the run with
// ErrProtocol.
func TestDiff_MalformedAnswers(t *testing.T) {
	local := newSet(tickets(3, "l")...)
	remote := newSet(append(tickets(3, "r"), ent("ticket/TKT-00009", "r"))...)
	first := func(resp *reconwire.Response) *reconwire.Result { return &resp.Ranges[0] }
	listed := func(f func(*reconwire.Result)) func(reconwire.Request, *reconwire.Response) {
		return func(req reconwire.Request, resp *reconwire.Response) {
			for i, rr := range req.Ranges {
				if rr.Op == "list" {
					f(&resp.Ranges[i])
				}
			}
		}
	}
	cases := map[string]func(reconwire.Request, *reconwire.Response){
		"too few answers": func(_ reconwire.Request, resp *reconwire.Response) { resp.Ranges = nil },
		"unknown status":  func(_ reconwire.Request, resp *reconwire.Response) { first(resp).Status = "maybe" },
		"short fingerprint": func(_ reconwire.Request, resp *reconwire.Response) {
			first(resp).Fingerprint = "abc"
		},
		"upper-case fingerprint": func(_ reconwire.Request, resp *reconwire.Response) {
			first(resp).Fingerprint = strings.ToUpper(first(resp).Fingerprint)
		},
		"no count": func(_ reconwire.Request, resp *reconwire.Response) { first(resp).Count = nil },
		"empty with a sum": func(_ reconwire.Request, resp *reconwire.Response) {
			zero := uint64(0)
			first(resp).Count = &zero
		},
		"bad hash":   listed(func(r *reconwire.Result) { r.Elements[0].Hash = "zz" }),
		"short hash": listed(func(r *reconwire.Result) { r.Elements[0].Hash = r.Elements[0].Hash[:62] }),
		"unordered": listed(func(r *reconwire.Result) {
			r.Elements[0], r.Elements[1] = r.Elements[1], r.Elements[0]
		}),
		"duplicate":   listed(func(r *reconwire.Result) { r.Elements[1] = r.Elements[0] }),
		"other type":  listed(func(r *reconwire.Result) { r.Elements[0].Address = "feature/FEAT-1" }),
		"no id":       listed(func(r *reconwire.Result) { r.Elements[0].Address = "ticket/" }),
		"control":     listed(func(r *reconwire.Result) { r.Elements[0].Address = "ticket/\x1b[31mX" }),
		"bidi":        listed(func(r *reconwire.Result) { r.Elements[0].Address = "ticket/\u202eX" }),
		"too long":    listed(func(r *reconwire.Result) { r.Elements[0].Address = "ticket/" + strings.Repeat("a", 5000) }),
		"over limit":  listed(func(r *reconwire.Result) { r.Elements = slices.Repeat(r.Elements, 600) }),
		"stuck page":  listed(func(r *reconwire.Result) { r.More, r.Last, r.Elements = true, "", nil }),
		"last behind": listed(func(r *reconwire.Result) { r.Last = "ticket/A" }),
		"too_large list": listed(func(r *reconwire.Result) {
			*r = reconwire.Result{Status: reconwire.StatusTooLarge}
		}),
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := diff(t, &fakeLocal{s: local}, &fakeRemote{s: remote, mutate: mutate},
				Scope{Entities: []string{"ticket"}}, Options{})
			if !errors.Is(err, ErrProtocol) {
				t.Errorf("err = %v, want ErrProtocol", err)
			}
		})
	}
}

// TestDiff_PageWithoutElements: an empty page that moves the cursor is
// valid (rows dropped when loaded); one that does not move it fails.
func TestDiff_PageWithoutElements(t *testing.T) {
	remote := newSet(tickets(3, "r")...)
	calls := 0
	advancing := func(req reconwire.Request, resp *reconwire.Response) {
		for i, rr := range req.Ranges {
			if rr.Op == "list" && rr.After == nil {
				calls++
				resp.Ranges[i] = reconwire.Result{Status: reconwire.StatusOK, Last: "ticket/TKT-00000", More: true}
			}
		}
	}
	res, err := diff(t, &fakeLocal{}, &fakeRemote{s: remote, mutate: advancing}, Scope{Entities: []string{"ticket"}},
		Options{})
	if err != nil || calls != 1 || len(res.OnlyRemote) != 2 {
		t.Fatalf("err=%v calls=%d only remote=%v", err, calls, addrs(res.OnlyRemote))
	}
	stuck := func(req reconwire.Request, resp *reconwire.Response) {
		for i, rr := range req.Ranges {
			if rr.Op == "list" && rr.After != nil {
				resp.Ranges[i] = reconwire.Result{Status: reconwire.StatusOK, Last: *rr.After, More: true}
			}
		}
	}
	if _, err := diff(t, &fakeLocal{}, &fakeRemote{s: newSet(tickets(2500, "r")...), mutate: stuck},
		Scope{Entities: []string{"ticket"}}, Options{}); !errors.Is(err, ErrProtocol) {
		t.Errorf("stuck cursor: err = %v", err)
	}
}

func TestDiff_RangeErrorPassesThrough(t *testing.T) {
	re := &RangeError{Index: 0, Status: 400, Detail: "bad"}
	_, err := diff(t, &fakeLocal{}, errRemote{re}, Scope{Entities: []string{"ticket"}}, Options{})
	if !errors.As(err, new(*RangeError)) {
		t.Errorf("err = %v", err)
	}
	gated := &RangeError{Index: 5, MetaGated: true, Status: 422}
	if _, err := diff(t, &fakeLocal{}, errRemote{gated}, Scope{Entities: []string{"ticket"}}, Options{}); !errors.Is(err, ErrProtocol) {
		t.Errorf("out-of-range index: err = %v", err)
	}
}

type errRemote struct{ err error }

func (e errRemote) Reconcile(context.Context, reconwire.Request) (reconwire.Response, error) {
	return reconwire.Response{}, e.err
}

func TestSanitize(t *testing.T) {
	if got := Sanitize("a\x1bb\u202ec"); got != "a�b�c" {
		t.Errorf("Sanitize = %q", got)
	}
	if got := Sanitize(strings.Repeat("x", 400)); len(got) != 303 {
		t.Errorf("long text kept %d bytes", len(got))
	}
	if Printable("a\u200bb") || !Printable("ticket/TKT-1@draft") || Printable("\xff") {
		t.Error("Printable misjudges")
	}
}
