// Package tokenstoretest is the conformance harness for [tokenstore.Store]
// implementations (TKT-01KZSO). Any new store must pass [RunAll].
//
// The clauses a store author could miss: a missing token fails with an error
// wrapping [tokenstore.ErrNotFound] (the broker turns that into "run rela
// token set"), deleting a missing token succeeds, names do not share state,
// and a token without a refresh token is refused rather than stored.
package tokenstoretest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

// Factory returns a fresh, empty store for one subtest.
type Factory func(tb testing.TB) tokenstore.Store

// RunAll runs every conformance test against stores from newStore.
func RunAll(t *testing.T, newStore Factory) {
	t.Helper()
	for name, fn := range map[string]func(*testing.T, Factory){
		"RoundTrip":               testRoundTrip,
		"GetMissingIsNotFound":    testGetMissing,
		"Overwrite":               testOverwrite,
		"DeleteRemoves":           testDeleteRemoves,
		"DeleteMissingIsNotError": testDeleteMissing,
		"NamesAreDistinct":        testNamesDistinct,
		"RefusesEmptyRefresh":     testRefusesEmptyRefresh,
		"LongValues":              testLongValues,
	} {
		t.Run(name, func(t *testing.T) { fn(t, newStore) })
	}
}

func name(t *testing.T, s string) tokenstore.Name {
	t.Helper()
	n, err := tokenstore.ParseName(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func sample() tokenstore.Token {
	return tokenstore.Token{
		Refresh:        "refresh-1",
		Access:         "access-1",
		ExpiresAt:      time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC),
		NeedsConsentAt: time.Date(2026, 4, 1, 9, 30, 0, 0, time.UTC),
	}
}

func equal(t *testing.T, got, want tokenstore.Token) {
	t.Helper()
	if got.Refresh != want.Refresh || got.Access != want.Access ||
		!got.ExpiresAt.Equal(want.ExpiresAt) || !got.NeedsConsentAt.Equal(want.NeedsConsentAt) {

		t.Fatalf("token = %+v, want %+v", got, want)
	}
}

func testRoundTrip(t *testing.T, newStore Factory) {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)
	n := name(t, "basecamp")
	if err := s.Put(ctx, n, sample()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, sample())

	// A token with only a refresh token round-trips with zero times.
	only := tokenstore.Token{Refresh: "r"}
	if err = s.Put(ctx, n, only); err != nil {
		t.Fatal(err)
	}
	got, err = s.Get(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, only)
	if !got.ExpiresAt.IsZero() || !got.NeedsConsentAt.IsZero() {
		t.Fatalf("zero times must stay zero: %+v", got)
	}
}

func testGetMissing(t *testing.T, newStore Factory) {
	t.Helper()
	_, err := newStore(t).Get(context.Background(), name(t, "absent"))
	if !errors.Is(err, tokenstore.ErrNotFound) {
		t.Fatalf("Get(missing) = %v, want ErrNotFound", err)
	}
}

func testOverwrite(t *testing.T, newStore Factory) {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)
	n := name(t, "crm")
	if err := s.Put(ctx, n, sample()); err != nil {
		t.Fatal(err)
	}
	next := tokenstore.Token{Refresh: "refresh-2", Access: "access-2"}
	if err := s.Put(ctx, n, next); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, next)
}

func testDeleteRemoves(t *testing.T, newStore Factory) {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)
	n := name(t, "crm")
	if err := s.Put(ctx, n, sample()); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, n); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, n); !errors.Is(err, tokenstore.ErrNotFound) {
		t.Fatalf("Get after Delete = %v, want ErrNotFound", err)
	}
}

func testDeleteMissing(t *testing.T, newStore Factory) {
	t.Helper()
	s := newStore(t)
	if err := s.Delete(context.Background(), name(t, "never-stored")); err != nil {
		t.Fatalf("Delete(missing) = %v, want nil", err)
	}
}

func testNamesDistinct(t *testing.T, newStore Factory) {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)
	a, b := name(t, "alpha"), name(t, "beta")
	if err := s.Put(ctx, a, tokenstore.Token{Refresh: "ra"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(ctx, b, tokenstore.Token{Refresh: "rb"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, a)
	if err != nil || got.Refresh != "ra" {
		t.Fatalf("Get(alpha) = %+v, %v", got, err)
	}
	if err := s.Delete(ctx, b); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, a); err != nil {
		t.Fatalf("deleting beta removed alpha: %v", err)
	}
}

func testRefusesEmptyRefresh(t *testing.T, newStore Factory) {
	t.Helper()
	if err := newStore(t).Put(context.Background(), name(t, "crm"), tokenstore.Token{Access: "a"}); err == nil {
		t.Fatal("Put without a refresh token must fail")
	}
}

// testLongValues stores tokens of the size real providers issue. A JWT
// access token can run past a kilobyte.
func testLongValues(t *testing.T, newStore Factory) {
	t.Helper()
	ctx := context.Background()
	s := newStore(t)
	n := name(t, "jwt")
	long := make([]byte, 1800)
	for i := range long {
		long[i] = 'a' + byte(i%('z'-'a'+1))
	}
	tok := tokenstore.Token{Refresh: string(long), Access: string(long[:1500])}
	if err := s.Put(ctx, n, tok); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, n)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, got, tok)
}
