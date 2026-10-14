package subscription

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/ingestspec"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// feedServer serves an RSS body that a test can change, with an optional
// ETag, and records the conditional headers it received.
type feedServer struct {
	mu          sync.Mutex
	body        string
	etag        string
	requests    int
	ifNoneMatch []string
	auth        []string
}

func (f *feedServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests++
	f.ifNoneMatch = append(f.ifNoneMatch, r.Header.Get("If-None-Match"))
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	if f.etag != "" {
		if r.Header.Get("If-None-Match") == f.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", f.etag)
	}
	w.Header().Set("Content-Type", "application/rss+xml")
	_, _ = w.Write([]byte(f.body))
}

func (f *feedServer) set(body, etag string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.body, f.etag = body, etag
}

// rss builds an RSS 2.0 feed of items given as guid=title pairs, newest
// first, dated one day apart.
func rss(items ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><rss version="2.0"><channel><title>T</title><link>/</link>`)
	for i, it := range items {
		guid, title, _ := strings.Cut(it, "=")
		fmt.Fprintf(&b, `<item><guid>%s</guid><title>%s</title><link>/p/%s</link>`+
			`<description>&lt;p&gt;%s&lt;/p&gt;</description><pubDate>%s</pubDate></item>`,
			guid, title, guid, title, fmt.Sprintf("Mon, %02d Sep 2026 10:00:00 GMT", 28-i))
	}
	b.WriteString(`</channel></rss>`)
	return b.String()
}

func articleSub(url string) Subscription {
	return Subscription{
		URL: url, Every: "1h",
		Mapping: ingestspec.Mapping{
			Find: &ingestspec.Find{Type: "article", Match: []string{"guid"}, Values: map[string]string{
				"guid": "{{item.id}}",
			}},
			CreateIfMissing: &ingestspec.Create{
				Properties: map[string]string{"title": "{{item.title}}", "link": "{{item.link}}"},
				Content:    "{{item.summary}}",
			},
			Then: []ingestspec.Step{{Set: map[string]string{"title": "{{item.title}}"}}},
		},
	}
}

func TestRun_ImportsThenSkipsUnchanged(t *testing.T) {
	srv := &feedServer{body: rss("b=Second", "a=First")}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w, kv := &world{}, &mapKV{}
	d := testDeps(w, kv, nil)
	sub := articleSub(ts.URL + "/feed")

	report, err := Run(t.Context(), "news", sub, d)
	if err != nil {
		t.Fatal(err)
	}
	if report.Created != 2 || w.creates != 2 {
		t.Fatalf("first poll report %+v, creates %d; want 2 created", report, w.creates)
	}
	// Oldest first, so creation order follows the feed's own chronology.
	if got := w.titles(); got[0] != "First" || got[1] != "Second" {
		t.Errorf("created %v, want oldest first", got)
	}
	created := w.rows[0]
	if created.Properties["link"] != ts.URL+"/p/a" {
		t.Errorf("link = %v, want resolved against the feed URL", created.Properties["link"])
	}
	if created.Content != "First" {
		t.Errorf("content = %q, want the summary as markdown", created.Content)
	}

	lists, patches := w.lists, w.patches
	report, err = Run(t.Context(), "news", sub, d)
	if err != nil {
		t.Fatal(err)
	}
	if report.Unchanged != 2 || w.lists != lists || w.patches != patches || w.creates != 2 {
		t.Errorf("second poll report %+v, lists +%d, patches +%d; want no store access",
			report, w.lists-lists, w.patches-patches)
	}
}

func TestRun_NotModifiedKeepsValidators(t *testing.T) {
	srv := &feedServer{body: rss("a=First"), etag: `"v1"`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w, kv := &world{}, &mapKV{}
	d := testDeps(w, kv, nil)
	sub := articleSub(ts.URL)

	for range 3 {
		if _, err := Run(t.Context(), "news", sub, d); err != nil {
			t.Fatal(err)
		}
	}
	if got := srv.ifNoneMatch; got[0] != "" || got[1] != `"v1"` || got[2] != `"v1"` {
		t.Errorf("If-None-Match sent %q, want the stored ETag after the first poll", got)
	}
	report, err := Run(t.Context(), "news", sub, d)
	if err != nil || !report.NotModified {
		t.Errorf("report %+v, err %v; want not modified", report, err)
	}
}

func TestRun_ChangedItemIsProcessedAgain(t *testing.T) {
	srv := &feedServer{body: rss("a=First")}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w := &world{}
	d := testDeps(w, &mapKV{}, nil)
	sub := articleSub(ts.URL)

	if _, err := Run(t.Context(), "news", sub, d); err != nil {
		t.Fatal(err)
	}
	srv.set(rss("a=First, edited"), "")
	report, err := Run(t.Context(), "news", sub, d)
	if err != nil {
		t.Fatal(err)
	}
	if report.Updated != 1 || w.titles()[0] != "First, edited" {
		t.Errorf("report %+v, titles %v; want the edit applied", report, w.titles())
	}
}

func TestRun_DeletedImportStaysDeleted(t *testing.T) {
	srv := &feedServer{body: rss("a=First")}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w := &world{}
	d := testDeps(w, &mapKV{}, nil)
	sub := articleSub(ts.URL)

	if _, err := Run(t.Context(), "news", sub, d); err != nil {
		t.Fatal(err)
	}
	w.delete(w.rows[0].ID)
	if _, err := Run(t.Context(), "news", sub, d); err != nil {
		t.Fatal(err)
	}
	if len(w.rows) != 0 {
		t.Errorf("rows %v, want the deleted import not recreated", w.titles())
	}
}

func TestRun_ConfigChangeResetsState(t *testing.T) {
	srv := &feedServer{body: rss("a=First"), etag: `"v1"`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w, kv := &world{}, &mapKV{}
	d := testDeps(w, kv, nil)

	if _, err := Run(t.Context(), "news", articleSub(ts.URL), d); err != nil {
		t.Fatal(err)
	}
	changed := articleSub(ts.URL + "/?v=2")
	if _, err := Run(t.Context(), "news", changed, d); err != nil {
		t.Fatal(err)
	}
	if got := srv.ifNoneMatch[1]; got != "" {
		t.Errorf("If-None-Match after a URL change = %q, want none", got)
	}
}

func TestRun_PermanentRejectionDoesNotFailTheRun(t *testing.T) {
	srv := &feedServer{body: rss("b=Bad", "a=Good"), etag: `"v1"`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w, kv := &world{}, &mapKV{}
	attempts := 0
	w.createErr = func(e *entity.Entity) error {
		if e.Properties["title"] == "Bad" {
			attempts++
			return &entitymanager.ValidationError{Errors: []*metamodel.ValidationError{{Message: "nope"}}}
		}
		return nil
	}
	d := testDeps(w, kv, nil)

	report, err := Run(t.Context(), "news", articleSub(ts.URL), d)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rejected != 1 || report.Created != 1 {
		t.Errorf("report %+v, want one rejected and one created", report)
	}
	srv.set(rss("b=Bad", "a=Good"), `"v2"`)
	if _, err := Run(t.Context(), "news", articleSub(ts.URL), d); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Errorf("rejected item tried %d times, want it recorded as seen", attempts)
	}
	if srv.ifNoneMatch[1] != `"v1"` {
		t.Errorf("If-None-Match = %q, want the validators saved", srv.ifNoneMatch[1])
	}
}

func TestRun_TransientFailureKeepsValidatorsAndProgress(t *testing.T) {
	srv := &feedServer{body: rss("b=Second", "a=First"), etag: `"v1"`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w, kv := &world{}, &mapKV{}
	failing := true
	w.createErr = func(e *entity.Entity) error {
		if failing && e.Properties["title"] == "Second" {
			return errors.New("disk full")
		}
		return nil
	}
	d := testDeps(w, kv, nil)
	sub := articleSub(ts.URL)

	if _, err := Run(t.Context(), "news", sub, d); err == nil {
		t.Fatal("Run succeeded, want the store error")
	}
	failing = false
	report, err := Run(t.Context(), "news", sub, d)
	if err != nil {
		t.Fatal(err)
	}
	if srv.ifNoneMatch[1] != "" {
		t.Errorf("If-None-Match after a failed run = %q, want none", srv.ifNoneMatch[1])
	}
	if report.Unchanged != 1 || report.Created != 1 {
		t.Errorf("report %+v, want the first item remembered and the second created", report)
	}
}

func TestRun_ConflictExhaustedIsNotSeen(t *testing.T) {
	srv := &feedServer{body: rss("a=First"), etag: `"v1"`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w := &world{}
	w.createErr = func(*entity.Entity) error { return store.ErrConflict }
	d := testDeps(w, &mapKV{}, nil)

	report, err := Run(t.Context(), "news", articleSub(ts.URL), d)
	if err != nil || report.Conflicted != 1 {
		t.Fatalf("report %+v, err %v; want one conflicted and no error", report, err)
	}
	w.createErr = nil
	report, err = Run(t.Context(), "news", articleSub(ts.URL), d)
	if err != nil || report.Created != 1 {
		t.Errorf("report %+v, err %v; want the item retried", report, err)
	}
	if got := srv.ifNoneMatch[1]; got != "" {
		t.Errorf("If-None-Match after a conflict = %q, want none so a 304 cannot hide the item", got)
	}
}

// TestRun_RebaselineSkipsSteps pins that a config change does not replay
// then: steps on entities the old config already handled, and that the
// steps resume once the record has been rebuilt.
func TestRun_RebaselineSkipsSteps(t *testing.T) {
	srv := &feedServer{body: rss("a=First")}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w, kv := &world{}, &mapKV{}
	d := testDeps(w, kv, nil)
	if _, err := Run(t.Context(), "news", articleSub(ts.URL), d); err != nil {
		t.Fatal(err)
	}

	changed := articleSub(ts.URL)
	changed.CreateIfMissing.Properties["guid"] = "{{item.id}}"
	srv.body = rss("b=Second", "a=First")
	patches := w.patches
	report, err := Run(t.Context(), "news", changed, d)
	if err != nil || report.Created != 1 || report.Unchanged != 1 || w.patches != patches {
		t.Fatalf("rebaseline report %+v, err %v, patches +%d; want the new item created and none patched",
			report, err, w.patches-patches)
	}

	srv.body = rss("b=Second", "a=First again")
	report, err = Run(t.Context(), "news", changed, d)
	if err != nil || report.Updated != 1 || w.patches != patches+1 {
		t.Errorf("report %+v, err %v, patches +%d; want the changed item's steps applied",
			report, err, w.patches-patches)
	}
}

func TestRun_SkipsItemWithoutIdentity(t *testing.T) {
	srv := &feedServer{body: `<rss version="2.0"><channel><item><title>no id</title></item></channel></rss>`}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w := &world{}
	report, err := Run(t.Context(), "news", articleSub(ts.URL), testDeps(w, &mapKV{}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if report.Skipped != 1 || len(w.rows) != 0 {
		t.Errorf("report %+v, rows %d; want the item skipped", report, len(w.rows))
	}
}

func TestRun_MaxItemsKeepsNewest(t *testing.T) {
	srv := &feedServer{body: rss("c=Third", "b=Second", "a=First")}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	w := &world{}
	sub := articleSub(ts.URL)
	sub.MaxItems = 2
	report, err := Run(t.Context(), "news", sub, testDeps(w, &mapKV{}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if report.Dropped != 1 || strings.Join(w.titles(), ",") != "Second,Third" {
		t.Errorf("report %+v, titles %v; want the two newest, oldest first", report, w.titles())
	}
}

func TestRun_SendsAuth(t *testing.T) {
	srv := &feedServer{body: rss("a=First")}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	sub := articleSub(ts.URL)
	sub.Auth = &Auth{Type: AuthBearer, Secret: "feed_token"}

	d := testDeps(&world{}, &mapKV{}, map[string]string{"feed_token": "s3cret"})
	if _, err := Run(t.Context(), "news", sub, d); err != nil {
		t.Fatal(err)
	}
	if srv.auth[0] != "Bearer s3cret" {
		t.Errorf("Authorization = %q", srv.auth[0])
	}

	d = testDeps(&world{}, &mapKV{}, nil)
	_, err := Run(t.Context(), "news", sub, d)
	if err == nil || !strings.Contains(err.Error(), `"feed_token" is not set`) {
		t.Errorf("err = %v, want the missing secret named", err)
	}
}

func TestRun_FailsOnBadFeed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer ts.Close()
	if _, err := Run(t.Context(), "news", articleSub(ts.URL), testDeps(&world{}, &mapKV{}, nil)); err == nil {
		t.Error("Run succeeded on a page that is not a feed")
	}
}

func TestRun_RejectsMissingDeps(t *testing.T) {
	full := testDeps(&world{}, &mapKV{}, nil)
	for name, mutate := range map[string]func(*Deps){
		"state":   func(d *Deps) { d.State = nil },
		"secrets": func(d *Deps) { d.Secrets = nil },
		"fetcher": func(d *Deps) { d.Fetcher = nil },
		"logger":  func(d *Deps) { d.Logger = nil },
	} {
		t.Run(name, func(t *testing.T) {
			d := full
			mutate(&d)
			if _, err := Run(t.Context(), "news", articleSub("http://x"), d); err == nil {
				t.Error("Run accepted missing deps")
			}
		})
	}
}
