package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// stubCopyService stands in for the manager. It records what it was asked so
// the tests can assert the handler does not pre-empt it.
type stubCopyService struct {
	offers []entitymanager.CopyOffer
	result *entitymanager.CopyResult
	err    error

	listCalls   int
	invokeCalls int
	gotReq      entitymanager.CopyRequest
}

func (s *stubCopyService) CopiesForSource(
	_ context.Context, _, _, _ string,
) ([]entitymanager.CopyOffer, error) {
	s.listCalls++
	return s.offers, s.err
}

func (s *stubCopyService) CopyState(
	_ context.Context, req entitymanager.CopyRequest,
) (*entitymanager.CopyResult, error) {
	s.invokeCalls++
	s.gotReq = req
	return s.result, s.err
}

func newCopyTestHandler(t *testing.T, svc copyService) *copiesHandler {
	t.Helper()
	app := newTestAppV1(t)
	h, err := newCopiesHandler(svc, app.State)
	if err != nil {
		t.Fatalf("newCopiesHandler: %v", err)
	}
	return h
}

func serveCopies(h *copiesHandler, method, target, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, http.NoBody)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	h.handleV1Copies(rec, r)
	return rec
}

// TestCopiesHandler_RejectsNilService pins the constructor rule.
//
// A nil service must fail at CONSTRUCTION, not per request, because an empty
// offer list is a legitimate domain answer — a face with no declared copies
// returns exactly that. So a nil-and-skip design would render a wiring bug as
// "the promote button never appears", sending someone to hunt through
// schema.yaml for a definition that is correctly declared.
func TestCopiesHandler_RejectsNilService(t *testing.T) {
	t.Parallel()
	app := newTestAppV1(t)
	if _, err := newCopiesHandler(nil, app.State); err == nil {
		t.Error("a nil copy service must be rejected at construction — a " +
			"wiring failure that renders as a valid domain answer is the " +
			"hardest kind to diagnose")
	}
	if _, err := newCopiesHandler(&stubCopyService{}, nil); err == nil {
		t.Error("a nil schema must be rejected at construction")
	}
}

// TestCopiesHandler_InvokeDoesNotPreEmptTheKernel is the load-bearing
// authorization test.
//
// The handler must NOT check the guard itself. Two authorization sites that
// can disagree is strictly worse than one, because the divergence is
// invisible — both look plausible. So the assertion is that the kernel is
// ALWAYS consulted, including for a request the handler might have been
// tempted to reject early.
//
// Mutation-checked: adding any pre-flight allow/deny check to invoke() fails
// this, because the stub's call count drops to zero.
func TestCopiesHandler_InvokeDoesNotPreEmptTheKernel(t *testing.T) {
	t.Parallel()
	svc := &stubCopyService{err: &acl.ForbiddenError{Decision: acl.Decision{
		RuleKind: "copy-guard", RuleID: "promote-page",
		Reason: "copy \"promote-page\" requires permission \"promote-page\" on \"PAGE-1\"",
	}}}
	h := newCopyTestHandler(t, svc)

	rec := serveCopies(h, http.MethodPost, "/api/v1/_copies/promote-page",
		`{"source_id":"PAGE-1"}`)

	if svc.invokeCalls != 1 {
		t.Fatalf("the kernel must be consulted exactly once, even for a request "+
			"that will be denied — the handler must not pre-empt it; calls=%d",
			svc.invokeCalls)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("a kernel ForbiddenError maps to 403; got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "promote-page") {
		t.Errorf("a 403 should name the missing permission — a copy definition "+
			"and its guard are operator config, not secrets; got %s", rec.Body)
	}
}

// TestCopiesHandler_MissingSourceIs404Indistinguishable is the existence-oracle
// test.
//
// The kernel folds a DENIED read on the source into ErrCopySourceMissing
// precisely so a caller cannot tell "you may not read this" from "this does
// not exist". The handler must preserve that: a 403 here — however much more
// helpful — would undo the read gate's work.
//
// It asserts BOTH the status and that the body carries no distinguishing
// detail, because a 404 whose body names the source would leak the same bit
// the status was careful not to.
func TestCopiesHandler_MissingSourceIs404Indistinguishable(t *testing.T) {
	t.Parallel()
	svc := &stubCopyService{
		err: fmt.Errorf("%w: PAGE-SECRET", entitymanager.ErrCopySourceMissing),
	}
	h := newCopyTestHandler(t, svc)

	rec := serveCopies(h, http.MethodPost, "/api/v1/_copies/promote-page",
		`{"source_id":"PAGE-SECRET"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("a missing-or-denied source must be 404, never 403: the kernel "+
			"collapses the two so a caller cannot tell them apart; got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "PAGE-SECRET") {
		t.Errorf("the 404 body must not name the source — that would leak "+
			"through the body the bit the status withheld; got %s", rec.Body)
	}
	if !strings.Contains(rec.Body.String(), entityNotFoundTitle) {
		t.Errorf("the 404 must use the shared not-found title so it is "+
			"byte-comparable with every other read-path 404; got %s", rec.Body)
	}
}

// TestCopiesHandler_OtherKernelErrorsAre422 covers the third arm: a coherent
// request this surface cannot satisfy (unknown definition, refused `when:`,
// cross-entity copy with no target).
func TestCopiesHandler_OtherKernelErrorsAre422(t *testing.T) {
	t.Parallel()
	svc := &stubCopyService{err: errors.New("copy \"nope\" is not declared")}
	h := newCopyTestHandler(t, svc)

	rec := serveCopies(h, http.MethodPost, "/api/v1/_copies/nope", `{"source_id":"PAGE-1"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("an ordinary kernel refusal maps to 422; got %d", rec.Code)
	}
}

// TestCopiesHandler_InvokesByNameOnly pins the transforms-registry precedent:
// the request names a DEFINITION, and the name comes from the PATH.
//
// If a caller could describe a copy, they could describe one whose guard is
// convenient and the guard system would be decorative. The body carries only
// entity ids, which is why entitymanager.CopyRequest is three strings.
func TestCopiesHandler_InvokesByNameOnly(t *testing.T) {
	t.Parallel()
	svc := &stubCopyService{result: &entitymanager.CopyResult{
		Definition: "promote-page",
		Entity:     &entity.Entity{ID: "PAGE-1", Type: "page", Pointer: entity.Pointer("published")},
		Created:    true,
	}}
	h := newCopyTestHandler(t, svc)

	rec := serveCopies(h, http.MethodPost, "/api/v1/_copies/promote-page",
		`{"source_id":"PAGE-1","fields":{"title":"injected"},"guard":null}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	// The definition came from the PATH; unknown body keys were ignored rather
	// than shaping the copy.
	if svc.gotReq.Definition != "promote-page" {
		t.Errorf("definition must come from the path; got %q", svc.gotReq.Definition)
	}
	if svc.gotReq.SourceID != "PAGE-1" {
		t.Errorf("source_id = %q", svc.gotReq.SourceID)
	}

	var got v1.CopyResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Pointer != "published" || !got.Created {
		t.Errorf("the result must report the written face and whether it was "+
			"created; got %+v", got)
	}
}

// TestCopiesHandler_ListReportsAllowedPerDefinition pins the config-is-not-
// secret rule on the wire: a denied definition is LISTED with allowed=false,
// not hidden.
//
// Copy names are operator-authored config, so concealing them would hide
// something schema.yaml already states. What is per-principal is the verdict.
func TestCopiesHandler_ListReportsAllowedPerDefinition(t *testing.T) {
	t.Parallel()
	svc := &stubCopyService{offers: []entitymanager.CopyOffer{
		{Name: "promote-page", Label: "Publish", TargetFace: "page@published",
			SameEntity: true, Allowed: true},
		{Name: "archive-page", Label: "archive-page", TargetFace: "page@archived",
			SameEntity: true, Allowed: false, Reason: "requires permission \"archive\""},
	}}
	h := newCopyTestHandler(t, svc)

	rec := serveCopies(h, http.MethodGet,
		"/api/v1/_copies?type=ticket&source_id=TKT-1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}

	var got v1.CopyOffersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("a DENIED definition must still be listed — its name is config, "+
			"not a secret; got %d entries", len(got.Data))
	}
	if !got.Data[0].Allowed || got.Data[1].Allowed {
		t.Errorf("allowed must be per-definition; got %+v", got.Data)
	}
	if got.Data[1].Reason == "" {
		t.Error("a denied offer should carry a reason for a tooltip")
	}
}

// TestCopiesHandler_ListValidatesItsAddress covers the request-shape errors,
// including that an unknown entity TYPE is named (config, not a secret) while
// nothing about entity existence is disclosed.
func TestCopiesHandler_ListValidatesItsAddress(t *testing.T) {
	t.Parallel()
	h := newCopyTestHandler(t, &stubCopyService{})

	for _, tc := range []struct {
		name, target string
		want         int
	}{
		{"no type", "/api/v1/_copies?source_id=TKT-1", http.StatusBadRequest},
		{"no source_id", "/api/v1/_copies?type=ticket", http.StatusBadRequest},
		{"unknown type", "/api/v1/_copies?type=nosuch&source_id=X-1", http.StatusNotFound},
		{"valid", "/api/v1/_copies?type=ticket&source_id=TKT-1", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := serveCopies(h, http.MethodGet, tc.target, "").Code; got != tc.want {
				t.Errorf("%s: got %d, want %d", tc.target, got, tc.want)
			}
		})
	}
}

// TestCopiesHandler_MethodRouting pins the surface's shape: a list is a GET on
// the collection, an invoke is a POST to a NAME. A POST to the collection has
// no definition to invoke, and a GET on a name is not a read of anything.
func TestCopiesHandler_MethodRouting(t *testing.T) {
	t.Parallel()
	h := newCopyTestHandler(t, &stubCopyService{})

	for _, tc := range []struct {
		name, method, target string
		want                 int
	}{
		{"POST to collection", http.MethodPost, "/api/v1/_copies", http.StatusMethodNotAllowed},
		{"GET a name", http.MethodGet, "/api/v1/_copies/promote-page", http.StatusMethodNotAllowed},
		{"DELETE", http.MethodDelete, "/api/v1/_copies/promote-page", http.StatusMethodNotAllowed},
		{"OPTIONS", http.MethodOptions, "/api/v1/_copies", http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := serveCopies(h, tc.method, tc.target, "").Code; got != tc.want {
				t.Errorf("%s %s: got %d, want %d", tc.method, tc.target, got, tc.want)
			}
		})
	}
}

// TestCopiesHandler_InvokeRejectsMalformedBody covers the wire-format errors,
// which are 400s rather than 422s: the request structure is broken, which is
// detectable without the metamodel (the DEC-HWZHA split).
func TestCopiesHandler_InvokeRejectsMalformedBody(t *testing.T) {
	t.Parallel()
	svc := &stubCopyService{}
	h := newCopyTestHandler(t, svc)

	if got := serveCopies(h, http.MethodPost, "/api/v1/_copies/x", `{not json`).Code; got != http.StatusBadRequest {
		t.Errorf("malformed JSON is a 400; got %d", got)
	}
	if got := serveCopies(h, http.MethodPost, "/api/v1/_copies/x", `{}`).Code; got != http.StatusBadRequest {
		t.Errorf("a missing source_id is a 400; got %d", got)
	}
	if svc.invokeCalls != 0 {
		t.Errorf("a malformed request must not reach the kernel; calls=%d", svc.invokeCalls)
	}
}
