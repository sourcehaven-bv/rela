package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Budget tests (collection-reads.md): a channel page and a channel list
// cost the same entity-store reads and header batches at 10 rows as at 50.
// Each message names another ticket, so a per-row ref lookup would show.

// channelBudget is an app whose channel service reads the entity store
// through a counter.
type channelBudget struct {
	app      *App
	d        *acl.Declarative
	ch       *channelService
	counting *storetest.Counting
}

// newChannelBudget seeds TKT-1 and n more tickets and wires a channel
// service over a counting store. alice reads and updates tickets.
func newChannelBudget(t *testing.T, n int) *channelBudget {
	t.Helper()
	app := newTestAppV1(t)
	seedEntity(app, ticket1("one"))
	for i := range n {
		seedEntity(app, &entity.Entity{
			ID: fmt.Sprintf("TKT-%d", 100+i), Type: "ticket", Properties: map[string]any{"title": fmt.Sprintf("t%d", i)},
		})
	}
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}, Update: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "editor"},
	}, app.store)
	setTestACL(t, app, d)
	counting := storetest.NewCounting(app.store)
	ch := newChannelService(t, app.Meta(), counting, d, channelTestRealtime)
	if err := SetChannels(app, ch.svc); err != nil {
		t.Fatal(err)
	}
	return &channelBudget{app: app, d: d, ch: ch, counting: counting}
}

// get serves path as alice and returns the store reads and header batches
// it cost.
func (b *channelBudget) get(t *testing.T, path string) (reads, headers int, body []byte) {
	t.Helper()
	b.counting.Reset()
	b.ch.headers.calls.Store(0)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	b.app.handleV1DynamicRoutes(rec, req.WithContext(gateCtxFor(aliceCtx(), t, b.d)))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body)
	}
	return b.counting.Reads(), int(b.ch.headers.calls.Load()), rec.Body.Bytes()
}

func TestChannelsBudget_Messages10vs50(t *testing.T) {
	cost := func(n int) (int, int) {
		b := newChannelBudget(t, n)
		ctx := gateCtxFor(aliceCtx(), t, b.d)
		addr := realtime.RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "room"}
		// The first post goes through the service, which records the
		// channel type and its refs; the rest go straight to the store,
		// past the post rate limit.
		if _, err := b.ch.svc.Post(ctx, addr, realtime.PostInput{
			Type: "text", Props: map[string]any{"body": "x", "about": "TKT-100"},
		}); err != nil {
			t.Fatal(err)
		}
		key := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
		for i := 1; i < n; i++ {
			if _, err := b.ch.store.Append(context.Background(), key, channels.NewMessage{
				Type: "text", Author: channels.Author{User: "alice"}, At: time.Now(),
				Props: map[string]any{"body": "x", "about": fmt.Sprintf("TKT-%d", 100+i)},
			}); err != nil {
				t.Fatal(err)
			}
		}
		reads, headers, body := b.get(t, fmt.Sprintf("%s/messages?limit=%d", roomPath, n))
		page := decodeAs[v1.MessagePage](t, body)
		if len(page.Messages) != n {
			t.Fatalf("page holds %d messages, want %d", len(page.Messages), n)
		}
		if ref, _ := page.Messages[n-1].Props["about"].(map[string]any); ref["title"] == nil {
			t.Fatalf("last message's ref did not resolve: %v", page.Messages[n-1].Props)
		}
		return reads, headers
	}
	r10, h10 := cost(10)
	r50, h50 := cost(50)
	if h10 == 0 {
		t.Fatal("no header batch counted; the counter is not on the read path")
	}
	if r10 != r50 || h10 != h50 {
		t.Errorf("a message page grows with rows: 10 = %d reads + %d header batches, 50 = %d + %d", r10, h10, r50, h50)
	}
}

func TestChannelsBudget_List10vs50(t *testing.T) {
	cost := func(n int) (int, int) {
		b := newChannelBudget(t, 0)
		anchor := channels.Anchor{ID: "TKT-1"}
		for i := range n {
			if _, err := b.ch.store.CreateMany(context.Background(), anchor, channels.NewChannel{
				Type: "threads", Title: fmt.Sprintf("thread %d", i), By: channels.Author{User: "alice"}, At: time.Now(),
			}); err != nil {
				t.Fatal(err)
			}
		}
		reads, headers, body := b.get(t, "/api/v1/tickets/TKT-1/_channels")
		for _, tl := range decodeAs[v1.ChannelList](t, body).Types {
			if tl.Type == "threads" && len(tl.Channels) != n {
				t.Fatalf("list holds %d threads, want %d", len(tl.Channels), n)
			}
		}
		return reads, headers
	}
	r10, h10 := cost(10)
	r50, h50 := cost(50)
	if h10 == 0 {
		t.Fatal("no header batch counted; the counter is not on the read path")
	}
	if r10 != r50 || h10 != h50 {
		t.Errorf("a channel list grows with rows: 10 = %d reads + %d header batches, 50 = %d + %d", r10, h10, r50, h50)
	}
}
