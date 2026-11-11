package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// flowControl is what `rela flow start|runs|cancel` use. The wiring site
// passes *appbuild.FlowControl, which applies the access checks.
type flowControl interface {
	Start(ctx context.Context, name, addr string) (flow.RunID, error)
	Cancel(ctx context.Context, id flow.RunID) error
	Runs(ctx context.Context, name string) ([]appbuild.FlowRunHeader, error)
}

// flowServices is the binding the run commands take.
//
// Nil Ctl: accepted; flows.yaml declares no flows, and every command says
// so.
type flowServices struct {
	Ctl flowControl
}

// errNoFlows answers a run command in a project without flows.
var errNoFlows = errors.New("flows.yaml declares no flows")

func (f *flowServices) ctl() (flowControl, error) {
	if f == nil || f.Ctl == nil {
		return nil, errNoFlows
	}
	return f.Ctl, nil
}

// FlowStartCmd starts a run by hand.
type FlowStartCmd struct {
	Flow    string `arg:"" help:"Flow name from flows.yaml."`
	Subject string `arg:"" help:"Subject entity ID, or ID@face."`
}

// Run starts the run and prints its id.
func (c *FlowStartCmd) Run(ctx context.Context, f *flowServices) error {
	ctl, err := f.ctl()
	if err != nil {
		return err
	}
	id, err := ctl.Start(ctx, c.Flow, c.Subject)
	if err != nil && id != "" {
		// The run exists but did not settle; the runtime retries it.
		return fmt.Errorf("started %s, but its first step failed and will be retried: %w", id, err)
	}
	if err != nil {
		return err
	}
	if out.Format == output.FormatJSON {
		return writeFlowJSON(map[string]string{"id": string(id)})
	}
	out.WriteMessage("Started %s", id)
	return nil
}

// FlowCancelCmd cancels an unfinished run.
type FlowCancelCmd struct {
	ID string `arg:"" help:"Run ID (the _flow-run entity)."`
}

// Run cancels the run.
func (c *FlowCancelCmd) Run(ctx context.Context, f *flowServices) error {
	ctl, err := f.ctl()
	if err != nil {
		return err
	}
	if err := ctl.Cancel(ctx, flow.RunID(c.ID)); err != nil {
		return err
	}
	out.WriteMessage("Cancelled %s", c.ID)
	return nil
}

// FlowRunsCmd lists runs.
type FlowRunsCmd struct {
	Flow string `help:"Only runs of this flow."`
}

// Run lists the run headers the caller can read, newest first.
func (c *FlowRunsCmd) Run(ctx context.Context, f *flowServices) error {
	ctl, err := f.ctl()
	if err != nil {
		return err
	}
	runs, err := ctl.Runs(ctx, c.Flow)
	if err != nil {
		return err
	}
	if out.Format == output.FormatJSON {
		if runs == nil {
			runs = []appbuild.FlowRunHeader{}
		}
		return writeFlowJSON(runs)
	}
	if len(runs) == 0 {
		out.WriteMessage("No runs.")
		return nil
	}
	tw := tabwriter.NewWriter(out.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tFLOW\tSTATE\tSTATUS\tSTARTED\tCOMPLETED")
	for _, r := range runs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, r.Flow, r.State, r.Status, r.Started, r.Completed)
	}
	return tw.Flush()
}

func writeFlowJSON(v any) error {
	enc := json.NewEncoder(out.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
