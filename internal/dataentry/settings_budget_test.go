package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// getSettingsAs loads the settings page as the principal on ctx.
func getSettingsAs(ctx context.Context, t *testing.T, app *App, d *acl.Declarative) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/_settings", http.NoBody)
	req = req.WithContext(gateCtxFor(ctx, t, d))
	rec := httptest.NewRecorder()
	app.appearance.handleAPIGetSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("_settings: %d %s", rec.Code, rec.Body)
	}
	return rec
}

var settingsOp budgetOp = func(t *testing.T, app *App, d *acl.Declarative, ctx context.Context) {
	t.Helper()
	getSettingsAs(ctx, t, app, d)
}

// bindOnlyOp binds the principal and does nothing else: the cost of the
// request edge, which every op pays.
var bindOnlyOp budgetOp = func(t *testing.T, _ *App, d *acl.Declarative, ctx context.Context) {
	t.Helper()
	gateCtxFor(ctx, t, d)
}

// The settings page used to list every readable entity of every relation
// target type as relation-default choices. It now picks a target through
// the search endpoint, so loading it costs the store nothing beyond binding
// the principal, at any store size (TKT-WJP6PBM).
func TestQueryBudget_SettingsReadsNoEntities(t *testing.T) {
	bind, _ := readsFor(t, bindOnlyOp)
	b, detail := readsFor(t, settingsOp)
	assertBudget(t, "_settings", b, bind.large.Calls, detail)
	if b.padLarge != bind.padLarge {
		t.Errorf("_settings cost %+v over the padded store, binding alone %+v: %s",
			b.padLarge, bind.padLarge, b.padDetail[1])
	}
}

// The response names each relation's target types and holds no target
// entity: neither the targets list nor a seeded feature's id or title.
func TestSettings_RelationDefsCarryTargetTypesOnly(t *testing.T) {
	app, _, d, ctx := newBudgetAppOn(t, 3, memstore.New())
	rec := getSettingsAs(ctx, t, app, d)

	body := rec.Body.String()
	for _, leak := range []string{`"targets"`, "Feature 1", `"F1"`} {
		if strings.Contains(body, leak) {
			t.Errorf("settings response holds %s: %s", leak, body)
		}
	}
	var data APISettingsData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("decode _settings: %v", err)
	}
	want := map[string][]string{"implements": {"feature"}, "tracked-by": {"epic", "ticket"}}
	for _, rd := range data.AllRelations {
		w, ok := want[rd.Name]
		if !ok {
			continue
		}
		delete(want, rd.Name)
		if !slices.Equal(rd.TargetTypes, w) || rd.TargetType != w[0] {
			t.Errorf("%s: targetType %q, targetTypes %v; want %q, %v", rd.Name, rd.TargetType, rd.TargetTypes, w[0], w)
		}
	}
	if len(want) != 0 {
		t.Errorf("relations missing from settings: %v", want)
	}
}
