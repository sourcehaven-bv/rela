package scheduler

import (
	"context"
	"errors"
	"reflect"
	"slices"
)

// WatchConfig makes the scheduler read schedules.yaml and subscriptions.yaml
// again on every tick, so an edit takes effect without a restart. A config
// that fails to load is not applied: the scheduler keeps the tasks it has, so
// a typo in one file does not stop the feeds and tasks that work.
//
// It also keeps Run ticking with no tasks, so a file added later is found.
// Call it before Run.
func (s *Scheduler) WatchConfig() {
	s.reload = func(ctx context.Context) (*Config, error) {
		return LoadWorkspaceConfig(ctx, s.ws)
	}
}

// reloadConfig applies a changed config. A run already queued for a removed
// task still reaches its handler: a script run carries its script in the
// payload and finishes, and a subscription poll fails as not configured.
func (s *Scheduler) reloadConfig(ctx context.Context) {
	if s.reload == nil {
		return
	}
	old := s.config.Load()
	cfg, err := s.reload(ctx)
	switch {
	case err != nil && errors.Is(err, ErrSubscriptions) && cfg != nil:
		// schedules.yaml loaded; only the feeds did not. Apply the
		// schedules and keep the running subscriptions, as at start.
		cfg = &Config{Tasks: withSubscriptionsOf(cfg.Tasks, old.Tasks)}
		s.noteReloadErr(err)
	case err != nil:
		s.noteReloadErr(err)
		return
	case s.reloadErr != "":
		s.reloadErr = ""
		s.logger.Info("scheduler config loads again")
	}
	if reflect.DeepEqual(old.Tasks, cfg.Tasks) {
		return
	}
	s.config.Store(cfg)
	added, removed, changed := taskDiff(old.Tasks, cfg.Tasks)
	s.logger.Info("scheduler config reloaded",
		"tasks", len(cfg.Tasks), "added", added, "removed", removed, "changed", changed)
}

// noteReloadErr logs err unless it is the error logged last.
func (s *Scheduler) noteReloadErr(err error) {
	if msg := err.Error(); msg != s.reloadErr {
		s.reloadErr = msg
		s.logger.Error("scheduler config not fully reloaded; keeping the running tasks it does not replace",
			"error", err)
	}
}

// withSubscriptionsOf returns tasks plus the subscription tasks of prev whose
// names tasks does not use.
func withSubscriptionsOf(tasks, prev []TaskConfig) []TaskConfig {
	out := slices.Clone(tasks)
	for _, t := range prev {
		if t.Subscription != nil && !slices.ContainsFunc(out, func(o TaskConfig) bool { return o.Name == t.Name }) {
			out = append(out, t)
		}
	}
	return out
}

// taskDiff returns the names in next but not in prev, the reverse, and the
// names in both whose settings differ.
func taskDiff(prev, next []TaskConfig) (added, removed, changed []string) {
	byName := func(tasks []TaskConfig) map[string]TaskConfig {
		out := make(map[string]TaskConfig, len(tasks))
		for _, t := range tasks {
			out[t.Name] = t
		}
		return out
	}
	before, after := byName(prev), byName(next)
	for _, t := range next {
		old, ok := before[t.Name]
		switch {
		case !ok:
			added = append(added, t.Name)
		case !reflect.DeepEqual(old, t):
			changed = append(changed, t.Name)
		}
	}
	for _, t := range prev {
		if _, ok := after[t.Name]; !ok {
			removed = append(removed, t.Name)
		}
	}
	return added, removed, changed
}
