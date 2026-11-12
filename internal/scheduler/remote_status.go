package scheduler

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/remotesync"
)

// RemoteStatus is what a status page shows about one remote: its settings,
// the types it maps with the conflict rule of each field, how its pulls
// went, and the rows its ledger links and its conflict list holds.
//
// Linked and Conflicts are operator state outside the graph and are not
// filtered by acl.yaml; a caller that shows them to a principal gates each
// row and field first, and counts after that.
type RemoteStatus struct {
	Name     string
	Settings remotesync.Settings
	Types    []RemoteType
	// LastRun is when the last successful pull started; zero when none has.
	LastRun time.Time
	// Running reports a pull, resolve or other run holding the remote now.
	Running bool
	// Failures counts the pulls that failed in a row since the last
	// success, and NextRetry is when the next attempt is due.
	Failures  int
	NextRetry time.Time
	// NextRun is when the schedule next makes a pull due. It is zero when
	// the remote has no schedule, or a pull is due now.
	NextRun   time.Time
	Linked    []remotesync.Linked
	Conflicts []remotesync.Conflict
}

// RemoteType is one type a remote maps: an entity type, or a relation type
// when Kind is [remotesync.KindRelation].
type RemoteType struct {
	Kind   string
	Local  string
	Source string
	Fields []RemoteField
}

// RemoteField is one mapped field and its conflict rule.
type RemoteField struct {
	Local, Remote, Rule string
}

// RemoteStatuses returns the status of every remote in remotes.yaml, sorted
// by name. A project without remotes.yaml has none.
//
// Nil: rejected for ws; the status lives in its state stores.
func RemoteStatuses(ctx context.Context, ws WorkspaceProvider) ([]RemoteStatus, error) {
	if ws == nil {
		return nil, errors.New("scheduler: a workspace is required")
	}
	var meta *metamodel.Metamodel
	if mp, ok := ws.(metaProvider); ok {
		meta = mp.Meta()
	}
	remotes, err := LoadRemotes(ctx, ws.Config(), meta)
	if err != nil {
		return nil, wrapRemotes(err)
	}
	names := remotes.Names()
	if len(names) == 0 {
		return nil, nil
	}
	runs := ws.SchedulerState()
	if runs == nil {
		return nil, errors.New("scheduler: the workspace has no scheduler state")
	}
	tasks := make([]string, len(names))
	for i, name := range names {
		tasks[i] = RemoteTaskPrefix + name
	}
	states, err := runs.Load(ctx, tasks)
	if err != nil {
		return nil, fmt.Errorf("scheduler: read run state: %w", err)
	}
	out := make([]RemoteStatus, 0, len(names))
	for _, name := range names {
		m := remotes.Mappings[name]
		ts := states[RemoteTaskPrefix+name]
		st := RemoteStatus{
			Name: name, Settings: m.Remote.Settings(), Types: remoteTypes(m),
			LastRun: ts.LastRun, Running: ts.Active != nil, Failures: ts.Failures, NextRetry: ts.NextRetry,
		}
		if every, perr := parseSchedule(m.Remote.Every); perr == nil && m.Remote.Every != "" && !ts.LastRun.IsZero() {
			st.NextRun = every.Next(ts.LastRun)
		}
		if st.Linked, err = remotesync.LoadLinked(ctx, ws.State(), m); err != nil {
			return nil, err
		}
		if st.Conflicts, err = remotesync.LoadConflicts(ctx, ws.State(), name); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// remoteTypes lists the types m maps, entities first, each sorted by source
// type.
func remoteTypes(m *remotesync.Mapping) []RemoteType {
	out := make([]RemoteType, 0, len(m.Entities)+len(m.Relations))
	for _, src := range slices.Sorted(maps.Keys(m.Entities)) {
		em := m.Entities[src]
		out = append(out, RemoteType{Local: em.Local, Source: em.Source, Fields: remoteFields(em.Fields)})
	}
	for _, src := range slices.Sorted(maps.Keys(m.Relations)) {
		rm := m.Relations[src]
		out = append(out, RemoteType{
			Kind: remotesync.KindRelation, Local: rm.Local, Source: rm.Source, Fields: remoteFields(rm.Fields),
		})
	}
	return out
}

func remoteFields(fields []remotesync.FieldMapping) []RemoteField {
	out := make([]RemoteField, len(fields))
	for i, f := range fields {
		out[i] = RemoteField{Local: f.Local.Name, Remote: f.Remote, Rule: f.Rule}
	}
	return out
}
