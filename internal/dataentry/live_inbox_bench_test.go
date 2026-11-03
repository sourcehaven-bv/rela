package dataentry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Sizes of BenchmarkLive_InboxRound (TKT-9YRRM5 AC 6).
const (
	benchInboxPrincipals = 20
	benchInboxTickets    = 500 // a case and a room channel each: 1000 channels
	// benchInboxFillEnv names the number of chat messages of about 1 KB
	// to put before each case's ask, so the waiting message sits in a
	// well-filled segment (TKT-V05706X AC 7). 0 by default: seeding 700
	// per case takes over an hour on the file store. The file store reads
	// the whole segment of each waiting message, so measure it with 700;
	// PostgreSQL reads each by its primary key, whatever the fill.
	benchInboxFillEnv = "RELA_BENCH_INBOX_FILL"
)

// benchInboxFill reads [benchInboxFillEnv].
func benchInboxFill(tb testing.TB) int {
	tb.Helper()
	v := os.Getenv(benchInboxFillEnv)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	require.NoError(tb, err, "%s", benchInboxFillEnv)
	return n
}

// BenchmarkLive_InboxRound times one full live round over 1000 channels
// and 20 principals that each hold a summary and a default page
// subscription. Each principal is waited on in 25 case channels, has a
// marker with unread messages on 25 rooms and is mentioned in 25 rooms.
// Each case holds RELA_BENCH_INBOX_FILL messages (0 by default) before its
// ask of about 4 KB, so the page's waiting-message step reads filled
// segments.
// The channel store is the file store on disk, or PostgreSQL with
// -tags postgres and RELA_TEST_DATABASE_URL set.
//
// It pins AC 6: one full inbox round finishes within its grid period,
// [LiveInboxGridScan] ticks (4 s) on the file store and [LiveInboxGrid]
// ticks (2 s) on a database store.
func BenchmarkLive_InboxRound(b *testing.B) {
	users := make([]string, benchInboxPrincipals)
	for i := range users {
		users[i] = fmt.Sprintf("p%02d", i)
	}
	policy := &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"editor": {Read: []string{"ticket", "person"}, Update: []string{"ticket"}, Permissions: []string{"inbox.read"}},
		},
		Assignments: map[string]string{},
	}
	for _, u := range users {
		policy.Assignments[u] = "editor"
	}
	ticket := func(i int) string { return fmt.Sprintf("TKT-%d", 1000+i) }
	cs, grid := benchChannelStore(b)
	env := newInboxLiveEnvOn(b, policy, func(app *App) {
		for _, u := range users {
			seedEntity(app, &entity.Entity{ID: u, Type: "person", Properties: map[string]any{"title": u}})
		}
		for i := range benchInboxTickets {
			seedEntity(app, &entity.Entity{ID: ticket(i), Type: "ticket",
				Properties: map[string]any{"title": ticket(i), "owner": users[i%benchInboxPrincipals]}})
		}
	}, cs, grid)
	ctx := context.Background()
	filler, question, fill := strings.Repeat("f", 1000), strings.Repeat("q", 4000), benchInboxFill(b)
	for i := range benchInboxTickets {
		owner, next := users[i%benchInboxPrincipals], users[(i+1)%benchInboxPrincipals]
		caseKey := channels.Key{Anchor: channels.Anchor{ID: ticket(i)}, Type: "case"}
		for range fill {
			_, err := env.ch.store.Append(ctx, caseKey, channels.NewMessage{
				Type: "chat", Author: channels.Author{User: next}, At: env.clock.now(),
				Props: map[string]any{"body": filler},
			})
			require.NoError(b, err)
		}
		benchPost(b, env, next, "/api/v1/tickets/"+ticket(i)+"/_channels/case",
			`{"type":"ask","props":{"body":"`+question+`"}}`)
		room := channels.Key{Anchor: channels.Anchor{ID: ticket(i)}, Type: "room"}
		benchPost(b, env, next, "/api/v1/tickets/"+ticket(i)+"/_channels/room",
			fmt.Sprintf(`{"type":"chat","props":{"body":"%s"}}`, "`@"+owner+"`"))
		_, _, err := env.ch.store.SetRead(ctx, room, channels.Reader{User: next}, 0, env.clock.now())
		require.NoError(b, err)
	}
	for _, u := range users {
		_, conn := env.open(b, u)
		env.subscribe(b, u, conn, liveSubEntry{"inbox", inboxPath}, liveSubEntry{"summary", inboxSummaryPath})
	}
	env.settle(b)
	env.ticks.through(b, uint64(grid-1)) // the next tick is a grid tick
	h := env.app.live
	h.inboxNow.Store(inboxTick{grid: true}.pack()) // replayRound runs off the worker: make this a grid tick

	b.ResetTimer()
	for b.Loop() {
		h.replayRound(ctx, true, nil)
	}
	b.StopTimer()
	if period, per := time.Duration(grid)*liveWindow, b.Elapsed()/time.Duration(b.N); per > period {
		b.Errorf("a full inbox round took %s, over its grid period of %s", per, period)
	}
	for _, u := range users {
		for _, s := range h.subsOf(u) {
			require.NotNil(b, s.wake, "every replay finished with a 200")
		}
	}
}

// benchPost posts body to path's messages as user and requires 201.
func benchPost(tb testing.TB, env *inboxLiveEnv, user, path, body string) {
	tb.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, env.srv.URL+path+"/messages",
		strings.NewReader(body))
	require.NoError(tb, err)
	req.Header.Set(testUserHeader, user)
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.srv.Client().Do(req)
	require.NoError(tb, err)
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	require.Equal(tb, http.StatusCreated, resp.StatusCode, "%s", got)
}
