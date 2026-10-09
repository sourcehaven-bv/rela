package tokenstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lock"
)

const (
	clientID     = "client-123"
	clientSecret = "SUPERSECRET-client"
)

// memStore is a minimal Store for broker tests; Sealed is covered by the
// conformance suite.
type memStore struct {
	mu   sync.Mutex
	toks map[Name]Token
	puts atomic.Int32
	// failPuts makes the next n Puts fail.
	failPuts atomic.Int32
}

func newMemStore() *memStore { return &memStore{toks: map[Name]Token{}} }

func (m *memStore) Get(_ context.Context, n Name) (Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.toks[n]
	if !ok {
		return Token{}, ErrNotFound
	}
	return t, nil
}

func (m *memStore) Put(ctx context.Context, n Name, t Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.failPuts.Add(-1) >= 0 {
		return errors.New("store unavailable")
	}
	m.failPuts.Store(0)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toks[n] = t
	m.puts.Add(1)
	return nil
}

func (m *memStore) Delete(_ context.Context, n Name) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.toks, n)
	return nil
}

type staticSecrets map[string]string

func (s staticSecrets) Secrets(string) (map[string]string, error) { return s, nil }

// provider is a token endpoint that rotates the refresh token on every
// refresh and refuses a stale one with invalid_grant, as Basecamp does.
type provider struct {
	t         *testing.T
	mu        sync.Mutex
	current   string
	calls     atomic.Int32
	style     Style
	expiresIn any
	status    int
	body      string
	hook      func()
	lastUA    string
}

func (p *provider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.calls.Add(1)
	if r.URL.RawQuery != "" {
		p.t.Errorf("credentials must travel in the body, got query %q", r.URL.RawQuery)
	}
	if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		p.t.Errorf("Content-Type = %q", ct)
	}
	if err := r.ParseForm(); err != nil {
		p.t.Error(err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastUA = r.UserAgent()
	if p.hook != nil {
		p.hook()
	}
	if p.status != 0 {
		w.WriteHeader(p.status)
		_, _ = w.Write([]byte(p.body))
		return
	}
	switch p.style {
	case StyleRFC6749:
		id, secret, ok := r.BasicAuth()
		if !ok || id != clientID || secret != clientSecret || r.PostForm.Has("client_secret") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	case StyleLaunchpad:
		if r.PostForm.Get("client_id") != clientID || r.PostForm.Get("client_secret") != clientSecret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	if r.PostForm.Get("grant_type") != "refresh_token" || r.PostForm.Get("refresh_token") != p.current {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}
	n := p.calls.Load()
	p.current = fmt.Sprintf("refresh-%d", n)
	resp := map[string]any{"access_token": fmt.Sprintf("access-%d", n), "refresh_token": p.current}
	if p.expiresIn != nil {
		resp["expires_in"] = p.expiresIn
	} else {
		resp["expires_in"] = 7200
	}
	_ = json.NewEncoder(w).Encode(resp)
}

type fixture struct {
	broker *Broker
	store  *memStore
	prov   *provider
	locker lock.Locker
	now    time.Time
}

func newFixture(t *testing.T, style Style) *fixture {
	t.Helper()
	prov := &provider{t: t, current: "refresh-0", style: style}
	srv := httptest.NewServer(prov)
	t.Cleanup(srv.Close)
	f := &fixture{store: newMemStore(), prov: prov, locker: lock.NewMemoryLocker(),
		now: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}
	b, err := NewBroker(BrokerDeps{
		Store:   f.store,
		Locker:  f.locker,
		Secrets: staticSecrets{"bc_id": clientID, "bc_secret": clientSecret},
		Connections: Connections{"basecamp": {
			Name: "basecamp", TokenURL: srv.URL + "/token", ClientIDSecret: "bc_id",
			ClientSecretSecret: "bc_secret", Style: style, UserAgent: "rela-test (ops@example.com)",
		}},
		Scope: "proj",
		Now:   func() time.Time { return f.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	f.broker = b
	f.store.toks["basecamp"] = Token{Refresh: "refresh-0"}
	return f
}

func TestNewBroker_RejectsMissingDeps(t *testing.T) {
	full := BrokerDeps{Store: newMemStore(), Locker: lock.NewMemoryLocker(), Secrets: staticSecrets{}, Scope: "s"}
	for name, mutate := range map[string]func(*BrokerDeps){
		"store":   func(d *BrokerDeps) { d.Store = nil },
		"locker":  func(d *BrokerDeps) { d.Locker = nil },
		"secrets": func(d *BrokerDeps) { d.Secrets = nil },
		"scope":   func(d *BrokerDeps) { d.Scope = "" },
	} {
		d := full
		mutate(&d)
		if _, err := NewBroker(d); err == nil {
			t.Errorf("missing %s accepted", name)
		}
	}
}

func TestBroker_RefreshesBothStyles(t *testing.T) {
	for _, style := range []Style{StyleRFC6749, StyleLaunchpad} {
		t.Run(string(style), func(t *testing.T) {
			f := newFixture(t, style)
			ctx := context.Background()
			tok, err := f.broker.AccessToken(ctx, "basecamp")
			if err != nil {
				t.Fatal(err)
			}
			if tok != "access-1" {
				t.Fatalf("access token = %q", tok)
			}
			stored := f.store.toks["basecamp"]
			if stored.Refresh != "refresh-1" || !stored.ExpiresAt.Equal(f.now.Add(2*time.Hour)) {
				t.Fatalf("stored %+v: the rotated refresh token and expiry must be kept", stored)
			}
			if f.prov.lastUA != "rela-test (ops@example.com)" {
				t.Errorf("User-Agent = %q", f.prov.lastUA)
			}

			// Fresh: no further call.
			if tok, err := f.broker.AccessToken(ctx, "basecamp"); err != nil || tok != "access-1" {
				t.Fatalf("second call = %q, %v", tok, err)
			}
			if n := f.prov.calls.Load(); n != 1 {
				t.Fatalf("provider calls = %d, want 1", n)
			}

			// Within the minute before expiry it refreshes again.
			f.now = f.now.Add(2*time.Hour - 30*time.Second)
			if tok, err := f.broker.AccessToken(ctx, "basecamp"); err != nil || tok != "access-2" {
				t.Fatalf("near expiry = %q, %v", tok, err)
			}
		})
	}
}

func TestBroker_ExpiresIn(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want time.Duration
	}{
		{"1800", 30 * time.Minute},
		{0, time.Hour},
		{json.RawMessage("null"), time.Hour},
		{1209600, 14 * 24 * time.Hour},
	} {
		f := newFixture(t, StyleLaunchpad)
		f.prov.expiresIn = tc.in
		if _, err := f.broker.AccessToken(context.Background(), "basecamp"); err != nil {
			t.Fatal(err)
		}
		if got := f.store.toks["basecamp"].ExpiresAt.Sub(f.now); got != tc.want {
			t.Errorf("expires_in %v: lifetime %v, want %v", tc.in, got, tc.want)
		}
	}
	f := newFixture(t, StyleLaunchpad)
	f.prov.expiresIn = "soon"
	if _, err := f.broker.AccessToken(context.Background(), "basecamp"); err == nil {
		t.Error("a non-numeric expires_in must fail")
	}
}

func TestParseExpiresIn(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      string
		want    time.Duration
		wantErr bool
	}{
		{name: "seconds", in: `90`, want: 90 * time.Second},
		{name: "string", in: `"90"`, want: 90 * time.Second},
		{name: "huge is a year", in: `1e300`, want: maxExpiry},
		{name: "huge string is a year", in: `"1e300"`, want: maxExpiry},
		{name: "a year exactly", in: `31536000`, want: maxExpiry},
		{name: "NaN", in: `"NaN"`, wantErr: true},
		{name: "Inf", in: `"Inf"`, wantErr: true},
		{name: "negative Inf", in: `"-Inf"`, wantErr: true},
		{name: "negative", in: `-5`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseExpiresIn(json.RawMessage(tc.in))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseExpiresIn(%s) = %v, want an error", tc.in, got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("parseExpiresIn(%s) = %v, %v; want %v", tc.in, got, err, tc.want)
			}
		})
	}
}

// R1: invalid_grant records needs_consent; later calls fail fast without
// calling the provider; Set clears it.
func TestBroker_InvalidGrantNeedsConsent(t *testing.T) {
	f := newFixture(t, StyleLaunchpad)
	f.store.toks["basecamp"] = Token{Refresh: "revoked"}
	ctx := context.Background()
	if _, err := f.broker.AccessToken(ctx, "basecamp"); !errors.Is(err, ErrNeedsConsent) {
		t.Fatalf("err = %v, want ErrNeedsConsent", err)
	}
	if f.store.toks["basecamp"].NeedsConsentAt.IsZero() {
		t.Fatal("needs_consent not recorded")
	}
	if _, err := f.broker.AccessToken(ctx, "basecamp"); !errors.Is(err, ErrNeedsConsent) {
		t.Fatalf("second call: %v", err)
	}
	if n := f.prov.calls.Load(); n != 1 {
		t.Fatalf("provider calls = %d, want 1: needs_consent must fail fast", n)
	}
	if err := f.broker.Set(ctx, "basecamp", Token{Refresh: "refresh-0"}); err != nil {
		t.Fatal(err)
	}
	if tok, err := f.broker.AccessToken(ctx, "basecamp"); err != nil || tok == "" {
		t.Fatalf("after set: %q, %v", tok, err)
	}
}

// R1: a caller canceled after the provider answered still stores the
// rotated token, because the old one is already dead.
func TestBroker_CancelAfterResponseStillStores(t *testing.T) {
	f := newFixture(t, StyleRFC6749)
	ctx, cancel := context.WithCancel(context.Background())
	f.prov.hook = cancel
	_, _ = f.broker.AccessToken(ctx, "basecamp")
	if got := f.store.toks["basecamp"].Refresh; got != "refresh-1" {
		t.Fatalf("stored refresh = %q, want the rotated refresh-1", got)
	}
}

// R1: error text never carries the URL, a body or a secret.
func TestBroker_ErrorsCarryNoSecrets(t *testing.T) {
	f := newFixture(t, StyleRFC6749)
	f.prov.status = http.StatusInternalServerError
	f.prov.body = `{"error":"server_error","echo":"refresh-0 ` + clientSecret + `"}`
	_, err := f.broker.AccessToken(context.Background(), "basecamp")
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "server_error") {
		t.Fatalf("err = %v", err)
	}
	assertClean(t, err, f.broker.conns["basecamp"].TokenURL)

	// A closed port.
	c := f.broker.conns["basecamp"]
	c.TokenURL = "http://127.0.0.1:1/token/path-marker"
	f.broker.conns["basecamp"] = c
	_, err = f.broker.AccessToken(context.Background(), "basecamp")
	if err == nil {
		t.Fatal("a closed port must fail")
	}
	assertClean(t, err, c.TokenURL)
}

func assertClean(t *testing.T, err error, tokenURL string) {
	t.Helper()
	msg := err.Error()
	for _, leak := range []string{clientSecret, clientID, "refresh-0", tokenURL, "/token", "echo"} {
		if strings.Contains(msg, leak) {
			t.Errorf("error %q contains %q", msg, leak)
		}
	}
}

// R1: a caller blocked on another holder's lock gives up without
// refreshing.
func TestBroker_BlockedWaiterTimesOut(t *testing.T) {
	f := newFixture(t, StyleLaunchpad)
	f.broker.lockWait = 50 * time.Millisecond
	release, err := f.locker.Acquire(context.Background(), "oauth-refresh/"+f.broker.lockScope+"/basecamp")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := f.broker.AccessToken(context.Background(), "basecamp"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want a deadline", err)
	}
	if n := f.prov.calls.Load(); n != 0 {
		t.Fatalf("provider calls = %d, want 0", n)
	}
}

// Invalidate, Set and Delete take the connection's lock too, so each gives
// up while another holder keeps it, and changes nothing.
func TestBroker_MutationsWaitForTheLock(t *testing.T) {
	for name, call := range map[string]func(*Broker) error{
		"Invalidate": func(b *Broker) error { return b.Invalidate(context.Background(), "basecamp", "access-old") },
		"Set": func(b *Broker) error {
			return b.Set(context.Background(), "basecamp", Token{Refresh: "refresh-new"})
		},
		"Delete": func(b *Broker) error { return b.Delete(context.Background(), "basecamp") },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, StyleLaunchpad)
			before := Token{Refresh: "refresh-0", Access: "access-old", ExpiresAt: f.now.Add(time.Hour)}
			f.store.toks["basecamp"] = before
			f.broker.lockWait = 50 * time.Millisecond
			release, err := f.locker.Acquire(context.Background(), "oauth-refresh/"+f.broker.lockScope+"/basecamp")
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if err := call(f.broker); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("err = %v, want a deadline", err)
			}
			if got := f.store.toks["basecamp"]; got != before {
				t.Fatalf("token changed without the lock: %+v", got)
			}
		})
	}
}

// A refreshed token is stored even when the first Put fails: the provider
// has already revoked the old refresh token.
func TestBroker_PersistRetriesOnce(t *testing.T) {
	f := newFixture(t, StyleLaunchpad)
	f.store.failPuts.Store(1)
	tok, err := f.broker.AccessToken(context.Background(), "basecamp")
	if err != nil || tok != "access-1" {
		t.Fatalf("AccessToken = %q, %v", tok, err)
	}
	if got := f.store.toks["basecamp"]; got.Refresh != "refresh-1" || got.Access != "access-1" {
		t.Fatalf("stored %+v, want the refreshed token", got)
	}

	// Two failures in a row are reported.
	f.now = f.now.Add(3 * time.Hour)
	f.store.failPuts.Store(2)
	_, err = f.broker.AccessToken(context.Background(), "basecamp")
	if err == nil || !strings.Contains(err.Error(), "storing the refreshed token") {
		t.Fatalf("err = %v, want the store failure", err)
	}
}

func TestBroker_ConnectionsIsACopy(t *testing.T) {
	f := newFixture(t, StyleLaunchpad)
	delete(f.broker.Connections(), "basecamp")
	if _, ok := f.broker.Connections()["basecamp"]; !ok {
		t.Fatal("a caller's change to Connections() reached the broker")
	}
}

// Concurrent callers on a stale token share one refresh.
func TestBroker_SingleRefreshUnderContention(t *testing.T) {
	f := newFixture(t, StyleLaunchpad)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Go(func() {
			if _, err := f.broker.AccessToken(context.Background(), "basecamp"); err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if n := f.prov.calls.Load(); n != 1 {
		t.Fatalf("provider calls = %d, want 1", n)
	}
}

// R2: invalidate clears only the token that was rejected.
func TestBroker_InvalidateComparesAndClears(t *testing.T) {
	f := newFixture(t, StyleLaunchpad)
	ctx := context.Background()
	f.store.toks["basecamp"] = Token{Refresh: "refresh-0", Access: "new", ExpiresAt: f.now.Add(time.Hour)}
	if err := f.broker.Invalidate(ctx, "basecamp", "old"); err != nil {
		t.Fatal(err)
	}
	if f.store.toks["basecamp"].Access != "new" {
		t.Fatal("a stale rejection cleared the current token")
	}
	if err := f.broker.Invalidate(ctx, "basecamp", "new"); err != nil {
		t.Fatal(err)
	}
	if got := f.store.toks["basecamp"]; got.Access != "" || got.Refresh != "refresh-0" {
		t.Fatalf("after invalidate: %+v", got)
	}
}

func TestBroker_NotConfiguredAndMissing(t *testing.T) {
	f := newFixture(t, StyleLaunchpad)
	ctx := context.Background()
	if _, err := f.broker.AccessToken(ctx, "jira"); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("undeclared: %v", err)
	}
	if _, err := f.broker.AccessToken(ctx, "Bad Name"); err == nil {
		t.Error("invalid name accepted")
	}
	delete(f.store.toks, "basecamp")
	_, err := f.broker.AccessToken(ctx, "basecamp")
	if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "rela token set basecamp") {
		t.Errorf("missing token: %v", err)
	}
	if err := f.broker.Set(ctx, "basecamp", Token{}); err == nil {
		t.Error("Set without a refresh token accepted")
	}
	if err := f.broker.Delete(ctx, "basecamp"); err != nil {
		t.Errorf("Delete(missing) = %v", err)
	}
}

func TestBroker_Status(t *testing.T) {
	f := newFixture(t, StyleLaunchpad)
	ctx := context.Background()
	st, err := f.broker.Status(ctx, "basecamp")
	if err != nil || !st.Stored || !st.Declared || st.AccessFresh {
		t.Fatalf("status = %+v, %v", st, err)
	}
	st, err = f.broker.Status(ctx, "other")
	if err != nil || st.Stored || st.Declared || st.Err != nil {
		t.Fatalf("status(other) = %+v, %v", st, err)
	}
}

func TestBroker_MissingClientSecret(t *testing.T) {
	f := newFixture(t, StyleLaunchpad)
	f.broker.secrets = staticSecrets{"bc_id": clientID}
	_, err := f.broker.AccessToken(context.Background(), "basecamp")
	if err == nil || !strings.Contains(err.Error(), "bc_secret") {
		t.Fatalf("err = %v, want it to name the missing secret", err)
	}
	if n := f.prov.calls.Load(); n != 0 {
		t.Fatalf("provider calls = %d", n)
	}
}
