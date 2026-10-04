package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/desktopnotify"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

const notifySchema = `version: "1.0"
entities:
  taak:
    label: Taak
    plural: taken
    id_prefix: "TAAK-"
    id_type: sequential
    display_property: title
    properties:
      title:
        type: string
      status:
        type: string
`

const notifyYAML = `notifications:
  - id: open
    type: taak
    condition: "entity.status == 'open'"
badge:
  type: taak
  condition: "entity.status == 'open'"
`

// recorder collects what a notifier delivers and the badge it last set.
type recorder struct {
	mu        sync.Mutex
	delivered []string
	badge     int
	badgeSet  chan int
}

func (r *recorder) deliver(_ string, m desktopnotify.Match) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.delivered = append(r.delivered, m.EntityID)
}

func (r *recorder) setBadge(_ string, n int) {
	r.mu.Lock()
	r.badge = n
	r.mu.Unlock()
	r.badgeSet <- n
}

func (r *recorder) snapshot() (titles []string, badge int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.delivered...), r.badge
}

func notifyKV(t *testing.T) state.KV {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewMemFS(), "/state")
	if err != nil {
		t.Fatal(err)
	}
	return state.NewFSKV(rfs)
}

func putTaak(t *testing.T, st *memstore.MemStore, id, status string) {
	t.Helper()
	e := &entity.Entity{ID: id, Type: "taak", Properties: map[string]any{"title": id, "status": status}}
	if err := st.CreateEntity(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}

// waitBadge waits for the next badge update, which every evaluation ends in.
func waitBadge(t *testing.T, r *recorder) int {
	t.Helper()
	select {
	case n := <-r.badgeSet:
		return n
	case <-time.After(5 * time.Second):
		t.Fatal("no evaluation within 5s")
		return 0
	}
}

func TestProjectNotifier_NotifiesNewMatchesAfterAWrite(t *testing.T) {
	meta, err := metamodel.Parse([]byte(notifySchema))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	putTaak(t, st, "TAAK-1", "open") // present before the first run: silent
	tracker, err := desktopnotify.NewTracker(notifyKV(t))
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{badgeSet: make(chan int, 16)}
	n, err := newProjectNotifier(projectNotifier{
		projectID: "p1",
		files:     fakeLoader{"desktop.yaml": notifyYAML},
		meta:      func() *metamodel.Metamodel { return meta },
		entities:  st,
		feed:      st,
		tracker:   tracker,
		deliver:   rec.deliver,
		badge:     rec.setBadge,
		interval:  time.Hour,
		debounce:  10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { n.run(ctx); close(done) }()

	if got := waitBadge(t, rec); got != 1 {
		t.Fatalf("first badge = %d, want 1", got)
	}
	if delivered, _ := rec.snapshot(); len(delivered) != 0 {
		t.Fatalf("first run delivered %v, want nothing", delivered)
	}

	putTaak(t, st, "TAAK-2", "open")
	if got := waitBadge(t, rec); got != 2 {
		t.Fatalf("badge after write = %d, want 2", got)
	}
	if delivered, _ := rec.snapshot(); len(delivered) != 1 || delivered[0] != "TAAK-2" {
		t.Fatalf("delivered %v, want [TAAK-2]", delivered)
	}

	cancel()
	<-done
	if _, badge := rec.snapshot(); badge != 0 {
		t.Fatalf("badge after close = %d, want 0", badge)
	}
}

func TestProjectNotifier_BadConfigClearsBadge(t *testing.T) {
	meta, err := metamodel.Parse([]byte(notifySchema))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	tracker, err := desktopnotify.NewTracker(notifyKV(t))
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{badgeSet: make(chan int, 4)}
	n, err := newProjectNotifier(projectNotifier{
		projectID: "p1",
		files:     fakeLoader{"desktop.yaml": "notifications: [ {id: x, type: nope, condition: 'true'} ]"},
		meta:      func() *metamodel.Metamodel { return meta },
		entities:  st, feed: st, tracker: tracker,
		deliver: rec.deliver, badge: rec.setBadge,
	})
	if err != nil {
		t.Fatal(err)
	}
	n.evaluate(context.Background())
	n.evaluate(context.Background())
	if got := waitBadge(t, rec); got != 0 {
		t.Fatalf("badge = %d, want 0", got)
	}
	if n.lastErr == "" {
		t.Fatal("load error not recorded")
	}
}

func TestNewProjectNotifier_RejectsMissing(t *testing.T) {
	if _, err := newProjectNotifier(projectNotifier{}); err == nil {
		t.Fatal("want error for missing project id")
	}
	if _, err := newProjectNotifier(projectNotifier{projectID: "p"}); err == nil {
		t.Fatal("want error for missing services")
	}
}

func TestNotificationTarget(t *testing.T) {
	p, path, ok := notificationTarget(map[string]any{"project": "p1", "type": "taak", "id": "A B"})
	if !ok || p != "p1" || path != "/entity/taak/A%20B" {
		t.Fatalf("notificationTarget = %q, %q, %v", p, path, ok)
	}
	if _, _, ok := notificationTarget(map[string]any{"project": "p1"}); ok {
		t.Fatal("incomplete data must not resolve")
	}
}
