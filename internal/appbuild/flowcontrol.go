package appbuild

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ErrFlowPermission answers a start or cancel by a principal that holds
// none of the flow's manual start permissions on the subject.
var ErrFlowPermission = errors.New("flows: permission denied")

// FlowControl starts, cancels and lists runs on behalf of the principal
// bound to ctx. It is the authorization layer above [flowengine.Engine],
// which trusts its caller.
//
// Reads go through the gated reader, so a subject or run header the
// caller cannot read answers not-found. A start or cancel needs a
// permission of one of the flow's `on: manual` starts, held for the
// subject (global roles and roles conferred through the graph). Without
// acl.yaml every principal may (the operator shell).
//
// The writes a start or cancel makes are audited with the caller as the
// flow step's By, so the log names who asked.
type FlowControl struct {
	eng  *flowengine.Engine
	d    *acl.Declarative
	rows interface {
		GetAddress(ctx context.Context, addr string) (*entity.Entity, error)
		ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
		ListRelationsStrict(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
	}
	now func() time.Time
}

// orphanGrace is how long a header may wait for its run before a cancel
// treats it as left behind: a start writes the header first.
const orphanGrace = time.Minute

// FlowRunHeader is one run as `rela flow runs` lists it, read from its
// `_flow-run` header.
type FlowRunHeader struct {
	ID        string `json:"id"`
	Flow      string `json:"flow"`
	Version   string `json:"version"`
	State     string `json:"state"`
	Status    string `json:"status"`
	Started   string `json:"started"`
	Completed string `json:"completed,omitempty"`
}

// FlowControlFor returns the flow control of s.
//
// Nil: returned when flows.yaml declares no flows.
func FlowControlFor(s *Services) *FlowControl {
	if s.flowEngine == nil {
		return nil
	}
	reader, _, _ := scriptReads(s.store, s.aclDeclarative, s.fieldReads, s.worlds)
	return &FlowControl{eng: s.flowEngine, d: s.aclDeclarative, rows: reader, now: time.Now}
}

// Start starts a run of the named flow on the subject at addr.
func (c *FlowControl) Start(ctx context.Context, name, addr string) (flow.RunID, error) {
	f, ok := c.eng.Flow(name)
	if !ok {
		return "", fmt.Errorf("%w: %q", flowengine.ErrUnknownFlow, name)
	}
	perms := manualPermissions(f)
	if len(perms) == 0 {
		return "", fmt.Errorf("flow %q has no `on: manual` start", name)
	}
	subject, err := c.read(ctx, addr)
	if err != nil {
		return "", err
	}
	if subject.Type != f.Subject() {
		return "", fmt.Errorf("flow %q runs on %s, and %s is a %s", name, f.Subject(), subject.ID, subject.Type)
	}
	if err := c.authorize(ctx, subject.ID, perms); err != nil {
		return "", err
	}
	return c.eng.Start(byCaller(ctx), name, subject.Ref())
}

// byCaller stamps the caller as the By of the flow step ctx carries.
func byCaller(ctx context.Context) context.Context {
	return audit.WithFlowStep(ctx, audit.FlowStep{By: principal.From(ctx).User})
}

// Cancel cancels the unfinished run id. The permissions are those of the
// manual starts of the flow version the run follows; for a run whose
// version no longer compiles, those of the current flow.
//
// A header left without a run (a crash between the two writes) is closed
// as cancelled once it is older than orphanGrace, on the same permissions.
func (c *FlowControl) Cancel(ctx context.Context, id flow.RunID) error {
	hdr, err := c.rows.GetAddress(ctx, string(id))
	if err != nil {
		if isNotFound(err) {
			return flowengine.ErrRunNotFound
		}
		return err
	}
	run, err := c.eng.Run(ctx, id)
	if errors.Is(err, flowengine.ErrRunNotFound) {
		return c.closeOrphan(ctx, hdr)
	}
	if err != nil {
		return err
	}
	var perms []string
	if f, ferr := c.eng.FlowOf(ctx, run); ferr == nil {
		perms = manualPermissions(f)
	} else if f, ok := c.eng.Flow(run.Flow); ok {
		perms = manualPermissions(f)
	}
	if err := c.authorize(ctx, run.Subject.ID, perms); err != nil {
		return err
	}
	return c.eng.Cancel(byCaller(ctx), id)
}

// closeOrphan closes the header hdr of a run that is not stored.
func (c *FlowControl) closeOrphan(ctx context.Context, hdr *entity.Entity) error {
	if flow.Status(propString(hdr, metamodel.FlowRunPropStatus)).Terminal() {
		return flowengine.ErrRunNotFound
	}
	// A backend may hand the datetime back as RFC 3339 text.
	started, err := time.Parse(time.RFC3339, propString(hdr, metamodel.FlowRunPropStarted))
	if err == nil && c.now().Sub(started) < orphanGrace {
		return fmt.Errorf("run %s is still starting; try again in a minute", hdr.ID)
	}
	var subject string
	for r, err := range c.rows.ListRelationsStrict(ctx, store.RelationQuery{
		From: hdr.ID, Type: entity.CarriesRelation,
	}) {
		if err != nil {
			return err
		}
		subject = r.To
	}
	if subject == "" {
		return flowengine.ErrRunNotFound
	}
	var perms []string
	if f, ok := c.eng.Flow(propString(hdr, metamodel.FlowRunPropFlow)); ok {
		perms = manualPermissions(f)
	}
	if err := c.authorize(ctx, subject, perms); err != nil {
		return err
	}
	return c.eng.CloseHeader(byCaller(ctx), flow.RunID(hdr.ID))
}

// Runs lists the run headers the caller can read, newest first. A
// non-empty name keeps one flow's runs.
func (c *FlowControl) Runs(ctx context.Context, name string) ([]FlowRunHeader, error) {
	var out []FlowRunHeader
	q := store.EntityQuery{Type: entity.FlowRunType, Faces: store.AllFaces()}
	for e, err := range c.rows.ListEntities(ctx, q) {
		if err != nil {
			return nil, err
		}
		h := FlowRunHeader{
			ID:        e.ID,
			Flow:      propString(e, metamodel.FlowRunPropFlow),
			Version:   propString(e, metamodel.FlowRunPropVersion),
			State:     propString(e, metamodel.FlowRunPropState),
			Status:    propString(e, metamodel.FlowRunPropStatus),
			Started:   propString(e, metamodel.FlowRunPropStarted),
			Completed: propString(e, metamodel.FlowRunPropCompleted),
		}
		if name != "" && h.Flow != name {
			continue
		}
		out = append(out, h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Started != out[j].Started {
			return out[i].Started > out[j].Started
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (c *FlowControl) read(ctx context.Context, addr string) (*entity.Entity, error) {
	e, err := c.rows.GetAddress(ctx, addr)
	if isNotFound(err) {
		return nil, fmt.Errorf("%w: %s", flowengine.ErrSubjectNotFound, addr)
	}
	return e, err
}

// authorize checks that the caller holds one of perms for the subject.
// Without acl.yaml it allows everything.
func (c *FlowControl) authorize(ctx context.Context, subjectID string, perms []string) error {
	if c.d == nil {
		return nil
	}
	req, err := c.d.RequestFrom(ctx)
	if err != nil {
		return err
	}
	for _, p := range perms {
		if req.HoldsPermissionForEntity(ctx, subjectID, p) {
			return nil
		}
	}
	if len(perms) == 0 {
		return fmt.Errorf("%w: the flow has no `on: manual` start", ErrFlowPermission)
	}
	return fmt.Errorf("%w: needs permission %v", ErrFlowPermission, perms)
}

func manualPermissions(f *flowcfg.Flow) []string {
	var out []string
	for _, s := range f.Starts() {
		if s.On == flowcfg.StartManual && s.Permission != "" {
			out = append(out, s.Permission)
		}
	}
	return out
}

func isNotFound(err error) bool {
	return errors.Is(err, store.ErrNotFound) || errors.Is(err, entitymanager.ErrEntityNotFound)
}

func propString(e *entity.Entity, key string) string {
	v, ok := e.Properties[key]
	if !ok || v == nil {
		return ""
	}
	switch v := v.(type) {
	case string:
		return v
	case time.Time:
		return v.UTC().Format(time.RFC3339)
	default:
		return fmt.Sprint(v)
	}
}
