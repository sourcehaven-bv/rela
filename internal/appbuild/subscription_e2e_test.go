package appbuild_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/scheduler"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const subscriptionFeed = `<?xml version="1.0"?>
<rss version="2.0"><channel><title>News</title>
<item><guid>g2</guid><title>Second ![](https://t.example/p.gif)</title><link>/p/2</link>
  <description>&lt;p&gt;Two &lt;script&gt;x()&lt;/script&gt;&lt;/p&gt;</description>
  <pubDate>Tue, 29 Sep 2026 10:00:00 GMT</pubDate></item>
<item><guid>g1</guid><title>First</title><link>/p/1</link>
  <description>&lt;p&gt;One&lt;/p&gt;</description>
  <pubDate>Mon, 28 Sep 2026 10:00:00 GMT</pubDate></item>
</channel></rss>`

// TestSubscription_EndToEnd runs a feed subscription through the real chain:
// the shared loader, the scheduler, the job queue, the gated write path and
// the state store. A second run gets a 304 and writes nothing.
func TestSubscription_EndToEnd(t *testing.T) {
	var requests, notModified atomic.Int32
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("If-None-Match") == `"v1"` {
			notModified.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(subscriptionFeed))
	}))
	defer feed.Close()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "schema.yaml"), []byte(`
entities:
  article:
    label: Article
    id_type: short
    id_prefix: "ART-"
    properties:
      title: {type: string, required: true}
      guid: {type: string}
      link: {type: string}
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "subscriptions.yaml"), []byte(`
subscriptions:
  news:
    url: `+feed.URL+`/feed.xml
    every: 1h
    find: {type: article, match: [guid], values: {guid: "{{item.id}}"}}
    create_if_missing:
      properties: {title: "{{item.title}}", link: "{{item.link}}"}
      content: "{{item.title}}\n\n{{item.summary}}"
`), 0o600))

	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	cfg, err := scheduler.LoadWorkspaceConfig(context.Background(), svc)
	require.NoError(t, err)
	// One scheduler for both runs: a Services carries at most one, since a
	// second registers the same job kinds.
	s, err := scheduler.NewWithQueue(cfg, script.NewEngine(), svc, discardTestLogger())
	require.NoError(t, err)

	runSubscriptionOnce(t, svc, s)
	articles := listArticles(t, svc)
	require.Equal(t, []string{"First", "Second ![](https://t.example/p.gif)"}, titlesOf(articles))
	for _, a := range articles {
		require.NotContains(t, a.Content, "script", "feed HTML must be sanitized")
	}
	require.Contains(t, articles[1].Content, `Second \!\[\](https://t.example/p.gif)`,
		"feed text in the body must be escaped, so it cannot place an image")
	require.Equal(t, feed.URL+"/p/1", articles[0].Properties["link"])

	// Forget the run so the task is due again, as runSchedulerTwice does.
	_, err = svc.SchedulerState().Prune(context.Background(), time.Now().Add(time.Hour))
	require.NoError(t, err)
	runSubscriptionOnce(t, svc, s)

	require.Equal(t, int32(2), requests.Load())
	require.Equal(t, int32(1), notModified.Load(), "the second poll must send the stored ETag")
	require.Len(t, listArticles(t, svc), 2)
}

func runSubscriptionOnce(t *testing.T, svc *appbuild.Services, s *scheduler.Scheduler) {
	t.Helper()
	runUntil(t, s, func() bool {
		got, err := svc.SchedulerState().Load(context.Background(), []string{"subscription:news"})
		if err != nil {
			return false
		}
		ts, ok := got["subscription:news"]
		return ok && ts.Active == nil && (!ts.LastRun.IsZero() || ts.Failures > 0)
	}, "the subscription did not finish a run")

	got, err := svc.SchedulerState().Load(context.Background(), []string{"subscription:news"})
	require.NoError(t, err)
	require.Zero(t, got["subscription:news"].Failures, "the poll failed")
}

type article struct {
	Content    string
	Properties map[string]any
}

func listArticles(t *testing.T, svc *appbuild.Services) []article {
	t.Helper()
	var out []article
	for e, err := range svc.Store().ListEntities(context.Background(), store.EntityQuery{
		Type: "article", Faces: store.InWorld(store.TrivialScope()),
	}) {
		require.NoError(t, err)
		out = append(out, article{Content: e.Content, Properties: e.Properties})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Properties["guid"].(string) < out[j].Properties["guid"].(string)
	})
	return out
}

func titlesOf(articles []article) []string {
	out := make([]string, len(articles))
	for i, a := range articles {
		out[i], _ = a.Properties["title"].(string)
	}
	return out
}

// TestSubscription_AddedWhileRunning pins the reload path through the real
// loader: a scheduler built before subscriptions.yaml exists polls it once
// the file appears, without a restart.
func TestSubscription_AddedWhileRunning(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(subscriptionFeed))
	}))
	defer feed.Close()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "schema.yaml"), []byte(`
entities:
  article:
    label: Article
    id_type: short
    id_prefix: "ART-"
    properties:
      title: {type: string, required: true}
      guid: {type: string}
`), 0o600))
	svc, err := discover(t, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	cfg, err := scheduler.LoadWorkspaceConfig(context.Background(), svc)
	require.NoError(t, err)
	require.Empty(t, cfg.Tasks)
	s, err := scheduler.NewWithQueue(cfg, script.NewEngine(), svc, discardTestLogger())
	require.NoError(t, err)
	s.WatchConfig()

	require.NoError(t, os.WriteFile(filepath.Join(root, "subscriptions.yaml"), []byte(`
subscriptions:
  news:
    url: `+feed.URL+`/feed.xml
    every: 1h
    find: {type: article, match: [guid], values: {guid: "{{item.id}}"}}
    create_if_missing:
      properties: {title: "{{item.title}}"}
`), 0o600))
	runSubscriptionOnce(t, svc, s)
	require.Len(t, listArticles(t, svc), 2)
}
