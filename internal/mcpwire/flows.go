package mcpwire

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/mcp"
)

// Flows registers the flow tools over the flow control of svc. Every read
// gate and role check stays in [appbuild.FlowControl]. A project without
// flows registers no tools.
func Flows(svc *appbuild.Services) mcp.Option {
	ctl := appbuild.FlowControlFor(svc)
	if ctl == nil {
		return mcp.WithFlows(nil)
	}
	return mcp.WithFlows(ctl)
}

// CurrentFlows registers the flow tools over the assembly current returns
// at each call, so a server that outlives a schema reload writes through
// the reloaded entity manager and metamodel. Whether the tools exist is
// decided once, by the assembly current returns now.
func CurrentFlows(current func() *appbuild.Services) mcp.Option {
	if appbuild.FlowControlFor(current()) == nil {
		return mcp.WithFlows(nil)
	}
	return mcp.WithFlows(currentFlows(current))
}

// currentFlows resolves the flow control per call.
type currentFlows func() *appbuild.Services

func (cf currentFlows) control() (*appbuild.FlowControl, error) {
	ctl := appbuild.FlowControlFor(cf())
	if ctl == nil {
		return nil, fmt.Errorf("%w: the project declares no flows", flowview.ErrNotFound)
	}
	return ctl, nil
}

func (cf currentFlows) Tasks(ctx context.Context, flowName string, page, perPage int) (flowview.Tasks, error) {
	ctl, err := cf.control()
	if err != nil {
		return flowview.Tasks{}, err
	}
	return ctl.Tasks(ctx, flowName, page, perPage)
}

func (cf currentFlows) Run(ctx context.Context, id flow.RunID) (flowview.Run, error) {
	ctl, err := cf.control()
	if err != nil {
		return flowview.Run{}, err
	}
	return ctl.Run(ctx, id)
}

func (cf currentFlows) Complete(ctx context.Context, id flow.RunID, gen int64,
	props map[string]any) (flowview.Completed, error) {
	ctl, err := cf.control()
	if err != nil {
		return flowview.Completed{}, err
	}
	return ctl.Complete(ctx, id, gen, props)
}
