package dataentry

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newDeclaredPilesApp is newPilesApp with an "inbox" pile declared: read
// state, read on open, one reason. Users may push to each other.
func newDeclaredPilesApp(t *testing.T) (*App, *acl.Declarative, *piles.Service) {
	t.Helper()
	app, d := facedAppWith(t, pilesTestMeta(t), policyPublishedScope(), func(st store.Store) *acl.Declarative {
		return pilesPolicy(t, st, "policy", "feature")
	})
	svc := newPilesServiceWith(t, piles.Options{
		OwnerExists: func(context.Context, string) (bool, error) { return true, nil },
		Declared: []piles.Declared{{
			Key: "inbox", Name: "Notifications", Icon: "bell",
			ReadState: true, AutoRead: true,
			Reasons: map[string]string{"done": "completed a task"},
		}},
	})
	if err := app.SetPiles(svc, stubScriptPiles{}); err != nil {
		t.Fatal(err)
	}
	return app, d, svc
}

// notifyAlice pushes refs onto alice's inbox as bob.
func notifyAlice(t *testing.T, svc *piles.Service, refs ...entity.Ref) {
	t.Helper()
	_, err := svc.Push(principalCtx("bob"), piles.PushRequest{
		Owner: "alice", Key: "inbox", Reason: "done", Refs: refs,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func inboxSummary(t *testing.T, app *App, d *acl.Declarative) v1.PileSummary {
	t.Helper()
	for _, p := range listPiles(t, app, pileCall{user: "alice", d: d}).Piles {
		if p.Key == "inbox" {
			return p
		}
	}
	t.Fatal("no inbox in the list")
	return v1.PileSummary{}
}

func TestDeclaredPiles_SummaryAndItems(t *testing.T) {
	app, d, svc := newDeclaredPilesApp(t)
	pol, err := entity.ParseRef("POL-1@published")
	if err != nil {
		t.Fatal(err)
	}
	feat, err := entity.ParseRef("FEAT-1")
	if err != nil {
		t.Fatal(err)
	}
	notifyAlice(t, svc, feat, pol)

	sum := inboxSummary(t, app, d)
	if !sum.Locked || !sum.ReadState || !sum.AutoRead || sum.Name != "Notifications" || sum.Icon != "bell" {
		t.Fatalf("summary = %+v", sum)
	}
	if sum.Unread != 2 || len(sum.UnreadAddresses) != 2 || len(sum.Addresses) != 2 {
		t.Fatalf("unread = %d %v, addresses %v", sum.Unread, sum.UnreadAddresses, sum.Addresses)
	}

	got := decodePile(t, doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodGet, path: pilesPath + "/" + sum.ID}))
	for _, it := range got.Items {
		if it.Read || it.Actor != "bob" || it.Reason != "completed a task" || it.Added == "" {
			t.Fatalf("item = %+v", it)
		}
	}

	// A hidden item is in no count and no address list.
	policyOnly := pilesPolicy(t, app.store, "policy")
	sum = inboxSummary(t, app, policyOnly)
	if sum.Count != 1 || sum.Unread != 1 || strings.Join(sum.Addresses, ",") != "POL-1@published" {
		t.Fatalf("gated summary = %+v", sum)
	}
}

func TestDeclaredPiles_MarkRead(t *testing.T) {
	app, d, svc := newDeclaredPilesApp(t)
	feat, _ := entity.ParseRef("FEAT-1")
	pol, _ := entity.ParseRef("POL-1@published")
	notifyAlice(t, svc, feat, pol)
	id := inboxSummary(t, app, d).ID

	post := func(path, body string) int {
		t.Helper()
		return doPile(t, app, pileCall{user: "alice", d: d, method: http.MethodPost, path: path, body: body}).Code
	}
	if code := post(pilesPath+"/"+id+"/items/_read", `{"items":["FEAT-1","nope"]}`); code != http.StatusNoContent {
		t.Fatalf("mark read: %d", code)
	}
	if sum := inboxSummary(t, app, d); sum.Unread != 1 || sum.UnreadAddresses[0] != "POL-1@published" {
		t.Fatalf("after mark read: %+v", sum)
	}
	// Opening an entity reads it on auto-read piles.
	if code := post(pilesPath+"/_read", `{"items":["POL-1@published"]}`); code != http.StatusNoContent {
		t.Fatalf("mark opened: %d", code)
	}
	if sum := inboxSummary(t, app, d); sum.Unread != 0 {
		t.Fatalf("after open: %+v", sum)
	}
	notifyAlice(t, svc, feat)
	if code := post(pilesPath+"/"+id+"/_read_all", ""); code != http.StatusNoContent {
		t.Fatalf("read all: %d", code)
	}
	if sum := inboxSummary(t, app, d); sum.Unread != 0 {
		t.Fatalf("after read all: %+v", sum)
	}

	// An ordinary pile keeps no read state.
	own := createPile(t, app, d, "alice", "Mine", "FEAT-1")
	if code := post(pilesPath+"/"+own.ID+"/_read_all", ""); code != http.StatusBadRequest {
		t.Fatalf("read all on own pile: %d", code)
	}
	// Another user's pile is the usual 404.
	rec := doPile(t, app, pileCall{user: "bob", d: d, method: http.MethodPost, path: pilesPath + "/" + id + "/_read_all"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign read all: %d %s", rec.Code, rec.Body)
	}
}

func TestDeclaredPiles_Locked(t *testing.T) {
	app, d, _ := newDeclaredPilesApp(t)
	id := inboxSummary(t, app, d).ID
	for _, c := range []pileCall{
		{method: http.MethodPatch, body: `{"name":"Other"}`},
		{method: http.MethodDelete},
	} {
		c.user, c.d, c.path = "alice", d, pilesPath+"/"+id
		rec := doPile(t, app, c)
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "pile_locked") {
			t.Fatalf("%s: %d %s", c.method, rec.Code, rec.Body)
		}
	}
}
