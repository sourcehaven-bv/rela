package main

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/desktopnotify"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Native notifications and the Dock badge, driven by a project's
// desktop.yaml (see internal/desktopnotify for the file and its rules).
//
// Each open project runs one projectNotifier. The Dock has one badge for the
// whole app, so it shows the sum over open projects.

const (
	// notifyInterval re-evaluates with no write at all, because a condition
	// on today() starts matching at midnight without anything changing.
	notifyInterval = 5 * time.Minute
	// notifyDebounce collapses a burst of writes (an import, a cascade) into
	// one evaluation.
	notifyDebounce = 2 * time.Second
	// notifyFeedBuffer is how many store events may queue before the feed
	// drops some; a dropped event only delays the next evaluation.
	notifyFeedBuffer = 64
)

// notifierServices is what startNotifier takes from a project's services.
type notifierServices interface {
	State() state.KV
	ProjectFiles() config.Loader
	Meta() *metamodel.Metamodel
	Store() store.Store
}

// changeFeed is the part of the store a notifier listens to.
type changeFeed interface {
	Subscribe(bufSize int) (events <-chan store.Event, cancel func())
}

// projectNotifier evaluates one project's desktop.yaml and reports what is
// new. The callbacks are the only way out, so the loop runs without Wails.
type projectNotifier struct {
	projectID string
	files     desktopnotify.ConfigLoader
	meta      func() *metamodel.Metamodel
	entities  store.EntityReader
	world     store.WorldScope // the world entities are read in
	feed      changeFeed
	tracker   *desktopnotify.Tracker

	deliver func(projectID string, m desktopnotify.Match)
	badge   func(projectID string, count int)

	interval, debounce time.Duration
	lastErr            string // the last load error logged, so it is logged once
}

// newProjectNotifier checks its collaborators; see the struct for each one.
func newProjectNotifier(n projectNotifier) (*projectNotifier, error) {
	switch {
	case n.projectID == "":
		return nil, errors.New("notifier: project id is required")
	case n.files == nil, n.meta == nil, n.entities == nil, n.feed == nil, n.tracker == nil, !n.world.IsSet():
		return nil, errors.New("notifier: project services are required")
	case n.deliver == nil, n.badge == nil:
		return nil, errors.New("notifier: deliver and badge callbacks are required")
	}
	if n.interval <= 0 {
		n.interval = notifyInterval
	}
	if n.debounce <= 0 {
		n.debounce = notifyDebounce
	}
	return &n, nil
}

// run evaluates now, then after each burst of store changes and on every
// interval, until ctx ends. It clears the project's badge on the way out.
func (n *projectNotifier) run(ctx context.Context) {
	events, cancel := n.feed.Subscribe(notifyFeedBuffer)
	defer cancel()
	defer n.badge(n.projectID, 0)

	n.evaluate(ctx)

	tick := time.NewTicker(n.interval)
	defer tick.Stop()
	settle := time.NewTimer(n.debounce)
	settle.Stop()
	defer settle.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				events = nil // the store closed its feed; keep the interval
				continue
			}
			settle.Reset(n.debounce)
		case <-settle.C:
			n.evaluate(ctx)
		case <-tick.C:
			n.evaluate(ctx)
		}
	}
}

// evaluate loads desktop.yaml afresh, so an edit applies on the next run
// without reopening the project.
func (n *projectNotifier) evaluate(ctx context.Context) {
	cfg, err := desktopnotify.Load(ctx, n.files, n.meta())
	if err != nil {
		if msg := err.Error(); msg != n.lastErr {
			n.lastErr = msg
			slog.Warn("desktop.yaml not applied", "project", n.projectID, "error", err)
		}
		n.badge(n.projectID, 0)
		return
	}
	n.lastErr = ""

	res, err := cfg.Evaluate(ctx, n.entities, n.world)
	if err != nil {
		slog.Warn("desktop notifications: evaluation failed", "project", n.projectID, "error", err)
		return
	}
	for _, e := range res.EvalErrors {
		slog.Debug("desktop notifications: condition failed", "project", n.projectID, "error", e)
	}

	// Update returns what it recorded even when it also fails, and those
	// will not be offered again, so they are delivered either way.
	fresh, err := n.tracker.Update(ctx, res)
	if err != nil {
		slog.Warn("desktop notifications: state not saved", "project", n.projectID, "error", err)
	}
	for _, m := range fresh {
		n.deliver(n.projectID, m)
	}

	count := 0
	if cfg.HasBadge() {
		count = res.Badge
	}
	n.badge(n.projectID, count)
}

// notifyHub is the app-wide side: the OS services, and the badge counts of
// every open project.
type notifyHub struct {
	notes *notifications.NotificationService
	dock  *dock.DockService

	authOnce sync.Once
	allowed  bool

	mu     sync.Mutex
	counts map[string]int
	shown  int
}

func newNotifyHub() *notifyHub {
	return &notifyHub{
		notes:  notifications.New(),
		dock:   dock.New(),
		counts: map[string]int{},
	}
}

// services are registered with the application at startup.
func (h *notifyHub) services() []application.Service {
	return []application.Service{application.NewService(h.notes), application.NewService(h.dock)}
}

// deliver shows one notification. The first one asks the user for
// permission; a refusal or an unbundled binary turns delivery off quietly.
// coverage-ignore-func: requires the OS notification center
func (h *notifyHub) deliver(projectID string, m desktopnotify.Match) {
	h.authOnce.Do(func() {
		ok, err := h.notes.RequestNotificationAuthorization()
		if err != nil {
			slog.Info("desktop notifications unavailable", "error", err)
		}
		h.allowed = ok && err == nil
	})
	if !h.allowed {
		return
	}
	err := h.notes.SendNotification(notifications.NotificationOptions{
		ID:    projectID + "/" + m.RuleID + "/" + m.EntityID,
		Title: m.Title,
		Body:  m.Body,
		Data:  map[string]any{"project": projectID, "type": m.EntityType, "id": m.EntityID},
	})
	if err != nil {
		slog.Warn("could not send notification", "error", err)
	}
}

// setBadge records one project's count and shows the total on the Dock.
// coverage-ignore-func: requires the Dock
func (h *notifyHub) setBadge(projectID string, count int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if count > 0 {
		h.counts[projectID] = count
	} else {
		delete(h.counts, projectID)
	}
	total := 0
	for _, c := range h.counts {
		total += c
	}
	if total == h.shown {
		return
	}
	h.shown = total
	var err error
	if total == 0 {
		err = h.dock.RemoveBadge()
	} else {
		err = h.dock.SetBadge(strconv.Itoa(total))
	}
	if err != nil {
		slog.Debug("could not update the Dock badge", "error", err)
	}
}

// notificationTarget is the project and entity page a clicked notification
// opens.
func notificationTarget(data map[string]any) (projectID, path string, ok bool) {
	projectID, _ = data["project"].(string)
	typ, _ := data["type"].(string)
	id, _ := data["id"].(string)
	if projectID == "" || typ == "" || id == "" {
		return "", "", false
	}
	return projectID, "/entity/" + url.PathEscape(typ) + "/" + url.PathEscape(id), true
}

// onNotificationClicked opens the entity a notification is about: in the
// main window when it shows that project, else in a window of its own.
// coverage-ignore-func: requires the OS notification center
func (d *Desktop) onNotificationClicked(result notifications.NotificationResult) {
	if result.Error != nil {
		return
	}
	projectID, path, ok := notificationTarget(result.Response.UserInfo)
	if !ok {
		return
	}
	if active := d.registry.activeProject(); active != nil && active.id == projectID && d.win != nil {
		d.win.Show()
		d.win.Focus()
		d.win.ExecJS(shellCommandJS(shellCommand{Command: "navigate", Arg: path}))
		return
	}
	if errMsg := d.OpenWindow(path, "", projectPrefix+projectID+"/"); errMsg != "" {
		slog.Warn("could not open notification target", "error", errMsg)
	}
}

// startNotifier runs desktop.yaml for a freshly loaded project until ctx,
// the project's lifetime, ends. A project the notifier cannot be built for
// still opens; it just sends nothing. world is the world entities are read in.
func (d *Desktop) startNotifier(
	ctx context.Context, projectID string, svc notifierServices, world store.WorldScope,
) {
	if d.notify == nil {
		return // no OS services (tests)
	}
	tracker, err := desktopnotify.NewTracker(svc.State())
	if err != nil {
		slog.Warn("desktop notifications disabled", "project", projectID, "error", err)
		return
	}
	n, err := newProjectNotifier(projectNotifier{
		projectID: projectID,
		files:     svc.ProjectFiles(),
		meta:      svc.Meta,
		entities:  svc.Store(),
		world:     world,
		feed:      svc.Store(),
		tracker:   tracker,
		deliver:   d.notify.deliver,
		badge:     d.notify.setBadge,
	})
	if err != nil {
		slog.Warn("desktop notifications disabled", "project", projectID, "error", err)
		return
	}
	go n.run(ctx)
}
