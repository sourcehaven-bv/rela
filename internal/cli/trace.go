package cli

import (
	"context"
	"fmt"
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
func (c *TraceFromCmd) Run(ctx context.Context, svc *cliServices) error {
	if _, err := svc.Store().GetEntity(ctx, c.ID); err != nil {
		return &entityNotFoundError{ID: c.ID}
	}
	result := svc.Tracer().TraceFrom(ctx, c.ID, c.Depth)
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
func (c *TraceToCmd) Run(ctx context.Context, svc *cliServices) error {
	if _, err := svc.Store().GetEntity(ctx, c.ID); err != nil {
		return &entityNotFoundError{ID: c.ID}
	}
	result := svc.Tracer().TraceTo(ctx, c.ID, c.Depth)
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
func (c *TracePathCmd) Run(ctx context.Context, svc *cliServices) error {
	if _, err := svc.Store().GetEntity(ctx, c.From); err != nil {
		return fmt.Errorf("source entity not found: %s", c.From)
	}
	if _, err := svc.Store().GetEntity(ctx, c.To); err != nil {
		return fmt.Errorf("target entity not found: %s", c.To)
	}
	path := svc.Tracer().FindPath(ctx, c.From, c.To)
	if path == nil {
		out.WriteMessage("No path found between %s and %s", c.From, c.To)
		return nil
	}
	return out.WritePath(path)
}
