package cli

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TraceCmd is the parent of trace from/to/path.
type TraceCmd struct {
	From TraceFromCmd `cmd:"" help:"Trace downstream dependencies."`
	To   TraceToCmd   `cmd:"" help:"Trace upstream dependencies."`
	Path TracePathCmd `cmd:"" help:"Find a path between two entities."`
}

// TraceFromCmd traces downstream dependencies from an entity.
type TraceFromCmd struct {
	ID    string `arg:"" help:"Source entity ID."`
	Depth int    `help:"Maximum depth to trace (0 = unlimited)."`
}

// Run dispatches `rela trace from <id>`.
func (c *TraceFromCmd) Run(ctx context.Context, svc *readServices) error {
	if err := requireFamily(ctx, svc.Store, c.ID); err != nil {
		return err
	}
	result := svc.Tracer.TraceFrom(ctx, c.ID, c.Depth)
	if result == nil {
		out.WriteMessage("No downstream dependencies found")
		return nil
	}
	return out.WriteTrace(result)
}

// TraceToCmd traces upstream dependencies of an entity.
type TraceToCmd struct {
	ID    string `arg:"" help:"Target entity ID."`
	Depth int    `help:"Maximum depth to trace (0 = unlimited)."`
}

// Run dispatches `rela trace to <id>`.
func (c *TraceToCmd) Run(ctx context.Context, svc *readServices) error {
	if err := requireFamily(ctx, svc.Store, c.ID); err != nil {
		return err
	}
	result := svc.Tracer.TraceTo(ctx, c.ID, c.Depth)
	if result == nil {
		out.WriteMessage("No upstream dependencies found")
		return nil
	}
	return out.WriteTrace(result)
}

// TracePathCmd finds a path between two entities.
type TracePathCmd struct {
	From  string `arg:"" help:"Source entity ID."`
	To    string `arg:"" help:"Target entity ID."`
	Depth int    `help:"Maximum depth to trace (0 = unlimited)."`
}

// Run dispatches `rela trace path <from> <to>`.
func (c *TracePathCmd) Run(ctx context.Context, svc *readServices) error {
	for _, end := range []struct{ role, id string }{{"source", c.From}, {"target", c.To}} {
		ok, err := familyExists(ctx, svc.Store, end.id)
		if err != nil {
			return fmt.Errorf("read %s entity %s: %w", end.role, end.id, err)
		}
		if !ok {
			return fmt.Errorf("%s entity not found: %s", end.role, end.id)
		}
	}
	path := svc.Tracer.FindPath(ctx, c.From, c.To)
	if path == nil {
		out.WriteMessage("No path found between %s and %s", c.From, c.To)
		return nil
	}
	return out.WritePath(path)
}

// familyExists reports whether id has a row on any face. A trace is entity
// level, and a faced type has no row at the bare id (BUG-95W7MV). A failed
// read is returned, so it is not reported as a missing entity.
func familyExists(ctx context.Context, st store.EntityLister, id string) (bool, error) {
	headers, err := store.FamilyHeaders(ctx, st, id)
	return len(headers) > 0, err
}

// requireFamily is [familyExists] as the not-found error the trace commands
// return.
func requireFamily(ctx context.Context, st store.EntityLister, id string) error {
	ok, err := familyExists(ctx, st, id)
	if err != nil {
		return fmt.Errorf("read entity %s: %w", id, err)
	}
	if !ok {
		return &entityNotFoundError{ID: id}
	}
	return nil
}
