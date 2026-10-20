package dataentry

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// Budget tests (collection-reads.md): a channel page and a channel list
// make the same entity-store reads, header batches included, and the same
// channel-store reads at 10 rows as at 50. Each message names another
// entity, half of them hidden from the reader, so a per-row ref lookup or a
// per-row fallback for a hidden ref would show.

// countingChannels counts the read calls of a channel store by method.
// Writes pass through uncounted, as in storetest.Counting.
type countingChannels struct {
	channels.Store
	mu    sync.Mutex
	calls map[string]int
}

func newCountingChannels(s channels.Store) *countingChannels {
	return &countingChannels{Store: s, calls: map[string]int{}}
}

func (c *countingChannels) hit(method string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls[method]++
}

// reset clears the counters.
func (c *countingChannels) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = map[string]int{}
}

// snapshot returns a copy of the counters.
func (c *countingChannels) snapshot() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return maps.Clone(c.calls)
}

func (c *countingChannels) Head(ctx context.Context, key channels.Key) (channels.Channel, error) {
	c.hit("Head")
	return c.Store.Head(ctx, key)
}

func (c *countingChannels) Get(ctx context.Context, key channels.Key, seq int64) (channels.Message, error) {
	c.hit("Get")
	return c.Store.Get(ctx, key, seq)
}

func (c *countingChannels) Messages(ctx context.Context, key channels.Key, page channels.Page) ([]channels.Message, error) {
	c.hit("Messages")
	return c.Store.Messages(ctx, key, page)
}

func (c *countingChannels) List(
	ctx context.Context, anchor channels.Anchor, typ string, state channels.State,
) ([]channels.Channel, error) {
	c.hit("List")
	return c.Store.List(ctx, anchor, typ, state)
}

func (c *countingChannels) ListEntity(ctx context.Context, id string) ([]channels.Channel, error) {
	c.hit("ListEntity")
	return c.Store.ListEntity(ctx, id)
}

func (c *countingChannels) ListByLifecycle(
	ctx context.Context, typ string, state channels.State, before time.Time,
) ([]channels.Channel, error) {
	c.hit("ListByLifecycle")
	return c.Store.ListByLifecycle(ctx, typ, state, before)
}

func (c *countingChannels) StalePending(ctx context.Context, before time.Time) ([]channels.PendingAction, error) {
	c.hit("StalePending")
	return c.Store.StalePending(ctx, before)
}

func (c *countingChannels) Audit(ctx context.Context, id string) ([]channels.AuditRecord, error) {
	c.hit("Audit")
	return c.Store.Audit(ctx, id)
}

func (c *countingChannels) Keys(ctx context.Context) ([]channels.Key, error) {
	c.hit("Keys")
	return c.Store.Keys(ctx)
}

func (c *countingChannels) TypeRecords(ctx context.Context) (map[string]channels.TypeRecord, error) {
	c.hit("TypeRecords")
	return c.Store.TypeRecords(ctx)
}

// channelBudget is an app whose channel service reads the entity store and
// the channel store through counters.
type channelBudget struct {
	app      *App
	d        *acl.Declarative
	ch       *channelService
	counting *storetest.Counting
	chans    *countingChannels
}

// channelCost is what one request cost: the entity-store reads by method
// (a header batch is one of them) and the channel-store reads by method.
type channelCost struct {
	entity, channel map[string]int
}

func (c channelCost) String() string {
	return fmt.Sprintf("entity store %v, channel store %v", c.entity, c.channel)
}

func (c channelCost) equal(o channelCost) bool {
	return maps.Equal(c.entity, o.entity) && maps.Equal(c.channel, o.channel)
}

// newChannelBudget seeds TKT-1 and n more tickets and n features, and wires
// a channel service over a counting entity store and a counting channel
// store. alice reads and updates tickets, and cannot read features.
func newChannelBudget(t *testing.T, n int) *channelBudget {
	t.Helper()
	return newChannelBudgetWith(t, n, channelTestRealtime)
}

// newChannelBudgetWith is newChannelBudget with yaml as realtime.yaml.
func newChannelBudgetWith(t *testing.T, n int, yaml string) *channelBudget {
	t.Helper()
	app := newTestAppV1(t)
	seedEntity(app, ticket1("one"))
	for i := range n {
		seedEntity(app, &entity.Entity{
			ID: fmt.Sprintf("TKT-%d", 100+i), Type: "ticket", Properties: map[string]any{"title": fmt.Sprintf("t%d", i)},
		})
		seedEntity(app, feature(fmt.Sprintf("FEAT-%d", 100+i), fmt.Sprintf("f%d", i)))
	}
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"editor": {Read: []string{"ticket"}, Update: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "editor"},
	}, app.store)
	setTestACL(t, app, d)
	cs, err := filechannels.New(storage.NewMemFS(), "/channels", filelock.ProcessPrivate{})
	if err != nil {
		t.Fatal(err)
	}
	chans := newCountingChannels(cs)
	counting := storetest.NewCounting(app.store)
	ch := newChannelServiceOver(t, app.Meta(), counting, d, yaml, chans)
	if err := SetChannels(app, ch.svc); err != nil {
		t.Fatal(err)
	}
	return &channelBudget{app: app, d: d, ch: ch, counting: counting, chans: chans}
}

// get serves path as alice and returns what it cost.
func (b *channelBudget) get(t *testing.T, path string) (cost channelCost, body []byte) {
	t.Helper()
	b.counting.Reset()
	b.chans.reset()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	b.app.handleV1DynamicRoutes(rec, req.WithContext(gateCtxFor(aliceCtx(), t, b.d)))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body)
	}
	return channelCost{entity: b.counting.Calls(), channel: b.chans.snapshot()}, rec.Body.Bytes()
}

// budgetRef is the entity message i names: a ticket alice reads for an even
// i, a feature she cannot read for an odd one.
func budgetRef(i int) string {
	if i%2 == 0 {
		return fmt.Sprintf("TKT-%d", 100+i)
	}
	return fmt.Sprintf("FEAT-%d", 100+i)
}

// messagesCost is what a page of n messages costs alice under yaml. Every
// message must offer want as its actions, next to the message operations.
func messagesCost(t *testing.T, n int, yaml string, want []string) channelCost {
	t.Helper()
	b := newChannelBudgetWith(t, n, yaml)
	ctx := gateCtxFor(aliceCtx(), t, b.d)
	addr := realtime.RawAddress{EntityType: "ticket", Anchor: "TKT-1", Type: "room"}
	// The first post goes through the service, which records the
	// channel type and its refs; the rest go straight to the store,
	// past the post rate limit and the check that a ref is readable.
	if _, err := b.ch.svc.Post(ctx, addr, realtime.PostInput{
		Type: "text", Props: map[string]any{"body": "x", "about": budgetRef(0)},
	}); err != nil {
		t.Fatal(err)
	}
	key := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	for i := 1; i < n; i++ {
		if _, err := b.ch.store.Append(context.Background(), key, channels.NewMessage{
			Type: "text", Author: channels.Author{User: "alice"}, At: time.Now(),
			Props: map[string]any{"body": "x", "about": budgetRef(i)},
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, body := b.get(t, fmt.Sprintf("%s/messages?limit=%d", roomPath, n))
	page := decodeAs[v1.MessagePage](t, body)
	if len(page.Messages) != n {
		t.Fatalf("page holds %d messages, want %d", len(page.Messages), n)
	}
	for _, m := range page.Messages {
		i := int(m.Seq) - 1
		ref, _ := m.Props["about"].(map[string]any)
		if i%2 == 0 {
			if ref["id"] != budgetRef(i) || ref["title"] == nil {
				t.Fatalf("message %d: readable ref = %v, want %s with a title", m.Seq, ref, budgetRef(i))
			}
			continue
		}
		if len(ref) != 1 || ref["restricted"] != true {
			t.Fatalf("message %d: hidden ref = %v, want only {restricted: true}", m.Seq, ref)
		}
	}
	for _, m := range page.Messages {
		// alice wrote every message: she may delete and edit each, also
		// one whose ref she cannot read (the edit keeps it).
		rowWant := append(slices.Clone(want), realtime.OpDelete, realtime.OpEdit)
		slices.Sort(rowWant)
		if !slices.Equal(m.Actions, rowWant) {
			t.Fatalf("message %d: _actions = %v, want %v", m.Seq, m.Actions, rowWant)
		}
	}
	return got
}

func TestChannelsBudget_Messages10vs50(t *testing.T) {
	cost := func(n int) channelCost { return messagesCost(t, n, channelTestRealtime, []string{"ack"}) }
	c10, c50 := cost(10), cost(50)
	if c10.entity["ListEntityHeaders"] == 0 || c10.channel["Messages"] == 0 {
		t.Fatalf("a counter is not on the read path: %s", c10)
	}
	if !c10.equal(c50) {
		t.Errorf("a message page grows with rows:\n10: %s\n50: %s", c10, c50)
	}
}

// TestChannelsBudget_ActionsReadNothing pins that the `_actions`
// affordance costs no store read: a page whose messages declare actions
// costs what the same page costs without them.
func TestChannelsBudget_ActionsReadNothing(t *testing.T) {
	without := strings.Replace(channelTestRealtime, `        actions:
          ack: { roles: [member] }
          accept: { roles: [member], promote_to: { type: ticket, map: { title: body } } }
`, "", 1)
	if without == channelTestRealtime {
		t.Fatal("the fixture no longer declares the actions this test removes")
	}
	with, bare := messagesCost(t, 10, channelTestRealtime, []string{"ack"}), messagesCost(t, 10, without, nil)
	if !with.equal(bare) {
		t.Errorf("_actions reads the store:\nwith actions: %s\nwithout:      %s", with, bare)
	}
}

func TestChannelsBudget_List10vs50(t *testing.T) {
	cost := func(n int) channelCost {
		b := newChannelBudget(t, 0)
		anchor := channels.Anchor{ID: "TKT-1"}
		for i := range n {
			if _, err := b.ch.store.CreateMany(context.Background(), anchor, channels.NewChannel{
				Type: "threads", Title: fmt.Sprintf("thread %d", i), By: channels.Author{User: "alice"}, At: time.Now(),
			}); err != nil {
				t.Fatal(err)
			}
		}
		got, body := b.get(t, "/api/v1/tickets/TKT-1/_channels")
		for _, tl := range decodeAs[v1.ChannelList](t, body).Types {
			if tl.Type == "threads" && len(tl.Channels) != n {
				t.Fatalf("list holds %d threads, want %d", len(tl.Channels), n)
			}
		}
		return got
	}
	c10, c50 := cost(10), cost(50)
	if c10.entity["ListEntityHeaders"] == 0 || len(c10.channel) == 0 {
		t.Fatalf("a counter is not on the read path: %s", c10)
	}
	if !c10.equal(c50) {
		t.Errorf("a channel list grows with rows:\n10: %s\n50: %s", c10, c50)
	}
}
