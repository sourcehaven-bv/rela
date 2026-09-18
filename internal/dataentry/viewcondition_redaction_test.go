package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// TestViewCondition_HiddenPropertyMakesCurrentUserConditionFalse is the list
// analog of TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse, and
// pins the IB-review finding on #1593.
//
// A list `condition:` used to be evaluated against the RAW stored properties,
// because field redaction happens later, at serialization. Since a condition
// may name is_current_user(entity.<field>), a condition over a field the
// reader cannot see decided whether the row appeared — so the hidden value
// leaked one bit at a time through row presence/absence, without the value
// ever being serialized.
//
// The next-action path already evaluates the REDACTED candidate for exactly
// this reason: redaction REMOVES a hidden property, it binds Nil, and every
// current-user form is false on Nil. Matching that here makes the condition
// strictly narrower than an unredacted evaluation, never wider.
func TestViewCondition_HiddenPropertyMakesCurrentUserConditionFalse(t *testing.T) {
	app := newTestAppV1(t)
	withTicketAssignment(t, app)
	app.Cfg().Lists["mine"] = dataentryconfig.List{
		EntityType: "ticket",
		Condition:  "is_current_user(entity.assignee)",
	}
	wireRealViewConditions(t, app)
	require.NoError(t, app.SetQueryScopeResolver(AdaptQueryScopes(appbuild.QueryScopes)))
	seedAssignedTicket(app, "TKT-alice", "alice")

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(principalCtx("alice"), t, d)

	// Baseline: with the assignee visible the row is a member of the list.
	// Without this the absence assertion below would pass against a list that
	// never matched anything.
	require.Equal(t, []string{"TKT-alice"},
		listIDsCtx(ctx, t, app, "/api/v1/tickets?list_id=mine"),
		"a visible assignee must satisfy the condition")

	// Hide the assignee. The reader may still READ the ticket, so this is
	// purely a field-level restriction — and the row must now drop out,
	// because a value the reader cannot see must not decide membership.
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"assignee": false}}}
	require.Empty(t, listIDsCtx(ctx, t, app, "/api/v1/tickets?list_id=mine"),
		"a hidden property must not satisfy a per-user condition: "+
			"row presence would leak the value one bit at a time")
}

// TestViewCondition_RedactionDoesNotWidenTheList pins the other direction.
//
// Redaction removes a property, so a NEGATED condition over a hidden field
// would start matching rows it previously excluded — turning a field
// restriction into a list that grows. The condition must only ever narrow.
func TestViewCondition_RedactionDoesNotWidenTheList(t *testing.T) {
	app := newTestAppV1(t)
	withTicketAssignment(t, app)
	app.Cfg().Lists["not_mine"] = dataentryconfig.List{
		EntityType: "ticket",
		Condition:  "not is_current_user(entity.assignee)",
	}
	wireRealViewConditions(t, app)
	require.NoError(t, app.SetQueryScopeResolver(AdaptQueryScopes(appbuild.QueryScopes)))
	seedAssignedTicket(app, "TKT-alice", "alice")
	seedAssignedTicket(app, "TKT-bob", "bob")

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d
	ctx := gateCtxFor(principalCtx("alice"), t, d)

	visible := listIDsCtx(ctx, t, app, "/api/v1/tickets?list_id=not_mine")
	require.Equal(t, []string{"TKT-bob"}, visible,
		"only the row assigned to someone else matches")

	// With the assignee hidden, `is_current_user` is false for BOTH rows, so
	// the negation is true for both and the list would GROW. That is the
	// accepted consequence of evaluating post-redaction: it reveals nothing
	// about the hidden value (every row is treated identically), whereas the
	// pre-redaction behavior revealed it per row.
	app.fieldResolver = fakeResolver{fv: FieldVerdicts{Visible: map[string]bool{"assignee": false}}}
	hidden := listIDsCtx(ctx, t, app, "/api/v1/tickets?list_id=not_mine")
	require.ElementsMatch(t, []string{"TKT-alice", "TKT-bob"}, hidden,
		"a hidden property binds Nil uniformly, so no row is distinguishable from another")
}

// listIDsCtx is listIDs with a caller-supplied context, so a test can drive
// the handler as a specific principal under a specific ACL gate.
func listIDsCtx(ctx context.Context, t *testing.T, app *App, url string) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "ticket", "tickets")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return idsFromListBody(t, rec)
}
