package dataentry

import (
	"encoding/json"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// A list may declare BOTH a `query_scope:` and a `condition:`. The two arrived
// on separate branches and neither side's tests could cover the pair, so this
// file is the seam between them.
//
// They are different mechanisms narrowing the same read. The scope decides
// membership from the entity TYPE's declared `query_scopes:` and is resolved
// before the store read; the condition is the VIEW's own expression and runs
// in Go over the rows that survived. Both must land before paging and the
// count, and each must still gate the pushdown fast path on its own — a page
// served straight from the store would skip whichever one has a Go-side
// remainder.

// scopeAndConditionApp wires the real compiler for BOTH features over the
// shared scope fixture, so this exercises schema + config -> compile -> resolve
// -> filter rather than two stubs that could agree with a broken merge.
func scopeAndConditionApp(t *testing.T, list dataentryconfig.List) *App {
	t.Helper()
	app := newScopeTestApp(t)
	app.Cfg().Lists["taken"] = list
	if err := app.SetViewConditions(
		AdaptViewConditions(appbuild.ViewConditions)); err != nil {
		t.Fatalf("wire view conditions: %v", err)
	}
	return app
}

// totalFromListBody reads meta.total, which must describe the same population
// the page was cut from.
func totalFromListBody(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	var resp struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Meta.Total
}

func sortedIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

// TestScopeAndCondition_BothNarrow is the core case: the result is the
// INTERSECTION, not whichever ran last.
//
// The fixture's three rows split cleanly. `archief` admits only TAAK-3, while
// the condition admits the two rows assigned to alice (TAAK-1, TAAK-3). Each
// narrowing alone yields a different answer from the pair, so a merge that
// dropped either one — or let one overwrite the other — fails here rather
// than passing on a set that happens to coincide.
func TestScopeAndCondition_BothNarrow(t *testing.T) {
	app := scopeAndConditionApp(t, dataentryconfig.List{
		EntityType: "taak",
		QueryScope: "archief",
		Condition:  "entity.toegewezen_aan == 'alice'",
	})

	rec := scopeListAs(app, "alice", "query_scope=archief&list_id=taken")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	got := idsFromListBody(t, rec)
	if want := []string{"TAAK-3"}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("scope ∩ condition = %v, want %v", got, want)
	}

	// Each half alone admits a DIFFERENT set, which is what makes the
	// assertion above a real intersection rather than a coincidence.
	scopeOnly := idsFromListBody(t, scopeListAs(app, "alice", "query_scope=archief"))
	if len(scopeOnly) != 1 || scopeOnly[0] != "TAAK-3" {
		t.Fatalf("scope alone = %v", scopeOnly)
	}
	condOnly := sortedIDs(idsFromListBody(t, scopeListAs(app, "alice", "query_scope=all&list_id=taken")))
	if len(condOnly) != 2 || condOnly[0] != "TAAK-1" || condOnly[1] != "TAAK-3" {
		t.Fatalf("condition alone = %v, want [TAAK-1 TAAK-3]", condOnly)
	}
}

// TestScopeAndCondition_IdentityWorksOnBothSides pins that `current_user`
// resolves for EACH mechanism when both are present.
//
// They reach identity by different routes: the scope stamps it once via
// scope.bind before the store read, the condition derives it per row from the
// request context inside the evaluator. Binding one does not bind the other,
// so a request whose scope AND condition both name the current user is the
// case where a shared-state assumption would show up.
func TestScopeAndCondition_IdentityWorksOnBothSides(t *testing.T) {
	app := scopeAndConditionApp(t, dataentryconfig.List{
		EntityType: "taak",
		QueryScope: "mijn",
		Condition:  "is_current_user(entity.toegewezen_aan)",
	})

	for _, tc := range []struct {
		user string
		want []string
	}{
		// alice owns TAAK-1 and TAAK-3; `mijn` is orthogonal to the archive,
		// so both survive when the condition names her too.
		{user: "alice", want: []string{"TAAK-1", "TAAK-3"}},
		{user: "bob", want: []string{"TAAK-2"}},
	} {
		t.Run(tc.user, func(t *testing.T) {
			rec := scopeListAs(app, tc.user, "query_scope=mijn&list_id=taken")
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			got := sortedIDs(idsFromListBody(t, rec))
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestCondition_CurrentUserWithoutAnyScope pins the guide's documented
// per-user condition on a list that names NO scope.
//
// GUIDE-data-entry.md ships exactly this shape:
//
//	condition: "is_current_user(entity.assignee) or entity.assignee == nil"
//
// It needs a query identity on ctx, and the only thing that stamped one was
// the scope helper — so a list with a condition and no scope raised
// ErrNoCurrentUser and answered 500 on every page. The documented example did
// not work, and nothing failed, because no test drove a `current_user`
// condition through the handler.
//
// `all` is used rather than omitting the parameter so the type's `default`
// scope does not resolve: this must pass with no scope in play at all.
func TestCondition_CurrentUserWithoutAnyScope(t *testing.T) {
	app := scopeAndConditionApp(t, dataentryconfig.List{
		EntityType: "taak",
		Condition:  "is_current_user(entity.toegewezen_aan)",
	})

	rec := scopeListAs(app, "bob", "query_scope=all&list_id=taken")
	if rec.Code != http.StatusOK {
		t.Fatalf("a condition naming current_user must not need a scope: %d %s",
			rec.Code, rec.Body.String())
	}
	got := idsFromListBody(t, rec)
	if len(got) != 1 || got[0] != "TAAK-2" {
		t.Fatalf("got %v, want [TAAK-2] (bob's row only)", got)
	}
}

// TestScopeAndCondition_TotalCountsTheDoublyNarrowedSet pins that the count
// describes the same population as the page when BOTH narrowings apply.
//
// Each is applied at a different point — the scope inside the store read, the
// condition after it — and only the second one's position was ever tested
// against the total. A count taken between them would report the scope's
// population for a page the condition shortened.
func TestScopeAndCondition_TotalCountsTheDoublyNarrowedSet(t *testing.T) {
	app := scopeAndConditionApp(t, dataentryconfig.List{
		EntityType: "taak",
		Condition:  "entity.toegewezen_aan == 'alice'",
	})

	// `all` withdraws the type's default so the scope contributes nothing,
	// isolating the count question to the condition over the full type.
	rec := scopeListAs(app, "alice", "query_scope=all&list_id=taken&per_page=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if got := idsFromListBody(t, rec); len(got) != 1 {
		t.Fatalf("per_page=1 must yield one row, got %v", got)
	}
	if total := totalFromListBody(t, rec); total != 2 {
		t.Fatalf("total = %d, want 2 (the conditioned population, not the type)", total)
	}
}
