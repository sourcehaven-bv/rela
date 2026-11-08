package reconclient

import (
	"cmp"
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// set is an in-memory element set in (Kind, Address) order. It answers the
// way the server's element read does, which both fakes below build on.
type set []reconcile.Element

func hashOf(s string) canonical.Digest { return canonical.Digest(sha256.Sum256([]byte(s))) }

func ent(addr, content string) reconcile.Element {
	return reconcile.Element{Kind: reconcile.KindEntity, Address: addr, Hash: hashOf(content)}
}

func rel(addr, content string) reconcile.Element {
	return reconcile.Element{Kind: reconcile.KindRelation, Address: addr, Hash: hashOf(content)}
}

func newSet(els ...reconcile.Element) set {
	s := slices.Clone(els)
	slices.SortFunc(s, func(a, b reconcile.Element) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Address, b.Address))
	})
	return s
}

// typeOf returns the type of an element's address.
func typeOf(e reconcile.Element) string {
	if e.Kind == reconcile.KindRelation {
		return strings.Split(e.Address, "--")[1]
	}
	t, _, _ := strings.Cut(e.Address, "/")
	return t
}

// in returns the elements of types in r.
func (s set) in(types visibility.ElementTypes, r store.ElementRange) []reconcile.Element {
	kind, names := reconcile.KindEntity, types.Entities
	if r.Relations {
		kind, names = reconcile.KindRelation, types.Relations
	}
	var out []reconcile.Element
	for _, e := range s {
		inRange := e.Address >= r.Lo && (r.Unbounded || e.Address < r.Hi)
		if e.Kind == kind && slices.Contains(names, typeOf(e)) && inRange {
			out = append(out, e)
		}
	}
	return out
}

// fakeLocal implements Local over a set. tooLarge refuses a fingerprint
// over more rows; gated refuses those relation types.
type fakeLocal struct {
	s         set
	tooLarge  int
	gated     []string
	listCalls int
	fpCalls   int
}

func (f *fakeLocal) refuse(types visibility.ElementTypes) error {
	for _, g := range f.gated {
		if slices.Contains(types.Relations, g) {
			return fmt.Errorf("%w: %q", visibility.ErrRelationMetaGated, g)
		}
	}
	return nil
}

func (f *fakeLocal) Fingerprint(
	_ context.Context, types visibility.ElementTypes, r store.ElementRange,
) (reconcile.Range, error) {
	f.fpCalls++
	if err := f.refuse(types); err != nil {
		return reconcile.Range{}, err
	}
	els := f.s.in(types, r)
	if f.tooLarge > 0 && len(els) > f.tooLarge {
		return reconcile.Range{}, visibility.ErrRangeTooLarge
	}
	var out reconcile.Range
	for _, e := range els {
		out = out.Add(e)
	}
	return out, nil
}

func (f *fakeLocal) List(
	_ context.Context, types visibility.ElementTypes, r store.ElementRange, limit int,
) (visibility.ElementPage, error) {
	f.listCalls++
	if err := f.refuse(types); err != nil {
		return visibility.ElementPage{}, err
	}
	els := f.s.in(types, r)
	page := visibility.ElementPage{}
	if len(els) > limit {
		els, page.More = els[:limit], true
	}
	page.Elements = els
	if len(els) > 0 {
		page.Last = els[len(els)-1].Address
	}
	return page, nil
}

// fakeRemote implements Remote over a set with the server's semantics and
// its limits. Hooks make it misbehave.
type fakeRemote struct {
	s set
	// tooLarge answers too_large for a fingerprint over more rows.
	tooLarge int
	// gated refuses a range naming one of these relation types with 422.
	gated []string
	// notRun answers not_run to the first notRun ranges.
	notRun int
	// refuse answers the first refuse requests with 429.
	refuse int
	// mutate rewrites an answer before it is returned.
	mutate func(req v1.ReconcileRequest, resp *v1.ReconcileResponse)

	requests []v1.ReconcileRequest
}

func (f *fakeRemote) Reconcile(_ context.Context, req v1.ReconcileRequest) (v1.ReconcileResponse, error) {
	f.requests = append(f.requests, req)
	if f.refuse > 0 {
		f.refuse--
		return v1.ReconcileResponse{}, &RefusedError{Status: 429, RetryAfter: 3e9}
	}
	if len(req.Ranges) > serverMaxRanges {
		return v1.ReconcileResponse{}, fmt.Errorf("fake: %d ranges", len(req.Ranges))
	}
	names, rows := 0, 0
	for _, rr := range req.Ranges {
		names += len(rr.Entities) + len(rr.Relations)
		rows += rr.Limit
	}
	if names > serverMaxTypeNames || rows > serverMaxListRows {
		return v1.ReconcileResponse{}, fmt.Errorf("fake: %d names, %d rows", names, rows)
	}
	for i, rr := range req.Ranges {
		for _, g := range f.gated {
			if slices.Contains(rr.Relations, g) {
				return v1.ReconcileResponse{}, &RangeError{Index: i, MetaGated: true, Status: 422, Detail: "gated " + g}
			}
		}
	}
	out := v1.ReconcileResponse{}
	for _, rr := range req.Ranges {
		out.Ranges = append(out.Ranges, f.answer(rr))
	}
	if f.mutate != nil {
		f.mutate(req, &out)
	}
	return out, nil
}

func (f *fakeRemote) answer(rr v1.ReconcileRange) v1.ReconcileResult {
	if f.notRun > 0 {
		f.notRun--
		return v1.ReconcileResult{Status: v1.ReconcileNotRun}
	}
	r := store.ElementRange{Relations: rr.Kind == "relation", Lo: rr.Lo, Unbounded: rr.Hi == nil}
	if rr.After != nil {
		r.Lo = *rr.After + "\x01"
	}
	if rr.Hi != nil {
		r.Hi = *rr.Hi
	}
	els := f.s.in(visibility.ElementTypes{Entities: rr.Entities, Relations: rr.Relations}, r)
	if rr.Op == "fingerprint" {
		if f.tooLarge > 0 && len(els) > f.tooLarge {
			return v1.ReconcileResult{Status: v1.ReconcileTooLarge}
		}
		var sum reconcile.Range
		for _, e := range els {
			sum = sum.Add(e)
		}
		n := sum.Count
		return v1.ReconcileResult{Status: v1.ReconcileOK, Count: &n, Fingerprint: sum.Sum.String()}
	}
	res := v1.ReconcileResult{Status: v1.ReconcileOK}
	if len(els) > rr.Limit {
		els, res.More = els[:rr.Limit], true
	}
	for _, e := range els {
		res.Elements = append(res.Elements, v1.ReconcileElement{Address: e.Address, Hash: e.Hash.String()})
	}
	if len(els) > 0 {
		res.Last = els[len(els)-1].Address
	}
	return res
}

// fpRanges counts the fingerprint ranges the remote was asked.
func (f *fakeRemote) fpRanges() int {
	n := 0
	for _, req := range f.requests {
		for _, rr := range req.Ranges {
			if rr.Op == "fingerprint" {
				n++
			}
		}
	}
	return n
}
