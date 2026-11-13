package mcp

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
)

// FlowSteps is what the flow tools need: the caller's tasks, one run with
// why it waits, and completing a step. The wiring site passes the
// project's *appbuild.FlowControl, which reads through the caller's row
// gate and checks the flow's roles itself; the tools only map arguments.
type FlowSteps interface {
	Tasks(ctx context.Context, flowName string, page, perPage int) (flowview.Tasks, error)
	Run(ctx context.Context, id flow.RunID) (flowview.Run, error)
	Complete(ctx context.Context, id flow.RunID, gen int64, props map[string]any) (flowview.Completed, error)
}

// WithFlows registers the flow tools (list_flow_tasks, show_flow_run,
// complete_flow_step). Without it they are absent, which is right for a
// project without flows.
//
// Nil: accepted — it registers no tools, so the wiring can pass what it
// has without a branch.
func WithFlows(f FlowSteps) Option {
	return func(s *Server) { s.flows = f }
}

// registerFlowTools adds the flow tools when the server was built
// [WithFlows].
func registerFlowTools(s *Server) {
	f := s.flows
	if f == nil {
		return
	}
	addTool(s, toolListFlowTasks(), func(ctx context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult,
		error) {
		return handleListFlowTasks(ctx, f, newToolRequest(req)), nil
	})
	addTool(s, toolShowFlowRun(), func(ctx context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult,
		error) {
		return handleShowFlowRun(ctx, f, newToolRequest(req)), nil
	})
	addTool(s, toolCompleteFlowStep(), func(ctx context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult,
		error) {
		return handleCompleteFlowStep(ctx, f, newToolRequest(req)), nil
	})
}

func toolListFlowTasks() *mcpgo.Tool {
	return newTool("list_flow_tasks",
		withDescription("List your flow tasks: runs that wait for a role you hold on their subject, oldest "+
			"first. Result: {tasks:[{run,flow,state,state_label,subject,started,branch}],total,truncated}. "+
			"A run in parallel branches gives one task per waiting branch. "+
			"Use show_flow_run for what a step needs"),
		withString("flow", description("Only this flow")),
		withNumber("page", description("Page, from 1 (default 1)")),
		withNumber("per_page", description("Tasks per page (default 50, at most 200)")),
	)
}

func toolShowFlowRun() *mcpgo.Tool {
	return newTool("show_flow_run",
		withDescription("Show a flow run and why it waits: the roles it waits for and whether you hold one, "+
			"the properties a step writes, its timers and due times, the change branches and whether "+
			"their guards hold now, a failing output step, and the timeline. `_actions.complete` says "+
			"whether complete_flow_step would be accepted. A run in parallel branches waits at its join and "+
			"lists each branch under `parallel`, with its own state, generation, waiting and "+
			"`_actions`-style complete flag"),
		withString("run", required(), description("Run id")),
	)
}

func toolCompleteFlowStep() *mcpgo.Tool {
	return newTool("complete_flow_step",
		withDescription("Complete the step a flow run waits for: write the given properties on its subject "+
			"as you, then move the run along the first change branch whose guard holds. You need a role "+
			"the state waits for, and every property must be one the state writes. Pass the generation "+
			"show_flow_run returned: a run that moved on since is refused. "+
			"Result: {written,moved,state,finished,problem}. problem is set when the properties were "+
			"written but the run did not follow cleanly: guard (a branch guard could not be evaluated, "+
			"so the run stays), moved (the run had already moved on) or settling (an output step "+
			"failed after the move and is retried while rela-server or the desktop app runs)"),
		withString("run", required(), description("Run id")),
		withNumber("generation", required(), description("The run's generation from show_flow_run, or "+
			"the branch's generation from its `parallel` entry")),
		withObject("properties", description("Property values to write; only names the state writes")),
	)
}

// maxFlowTasksPage caps per_page of list_flow_tasks, and
// defaultFlowTasksPage is its default.
const (
	maxFlowTasksPage     = 200
	defaultFlowTasksPage = 50
)

// flowErrorResult passes on the text of an error with a [flowview] kind,
// which is written for the caller. Any other error is an internal failure:
// it goes to the log, and the caller gets a fixed text.
func flowErrorResult(ctx context.Context, err error) *mcpgo.CallToolResult {
	for _, kind := range []error{flowview.ErrNotFound, flowview.ErrConflict, flowview.ErrForbidden,
		flowview.ErrInvalid} {
		if errors.Is(err, kind) {
			return errorResult(err.Error())
		}
	}
	slog.ErrorContext(ctx, "mcp: flow tool failed", "error", err)
	return errorResult("the flow call failed; see the server log")
}

func handleListFlowTasks(ctx context.Context, f FlowSteps, args toolRequest) *mcpgo.CallToolResult {
	perPage := min(args.GetInt("per_page", defaultFlowTasksPage), maxFlowTasksPage)
	out, err := f.Tasks(ctx, strings.TrimSpace(args.GetString("flow", "")), args.GetInt("page", 1), perPage)
	if err != nil {
		return flowErrorResult(ctx, err)
	}
	return jsonResult(out)
}

func handleShowFlowRun(ctx context.Context, f FlowSteps, args toolRequest) *mcpgo.CallToolResult {
	id, err := args.RequireString("run")
	if err != nil {
		return errorResult(err.Error())
	}
	out, err := f.Run(ctx, flow.RunID(strings.TrimSpace(id)))
	if err != nil {
		return flowErrorResult(ctx, err)
	}
	return jsonResult(out)
}

func handleCompleteFlowStep(ctx context.Context, f FlowSteps, args toolRequest) *mcpgo.CallToolResult {
	id, err := args.RequireString("run")
	if err != nil {
		return errorResult(err.Error())
	}
	gen, ok := args.GetArguments()["generation"].(float64)
	if !ok || gen < 0 || gen != float64(int64(gen)) {
		return errorResult("generation must be a whole number from show_flow_run")
	}
	var props map[string]any
	switch v := args.GetArguments()["properties"].(type) {
	case nil:
	case map[string]any:
		props = v
	default:
		return errorResult("properties must be an object")
	}
	out, err := f.Complete(ctx, flow.RunID(strings.TrimSpace(id)), int64(gen), props)
	if err != nil {
		if errors.Is(err, flowview.ErrConflict) {
			return errorResult(err.Error() + "; call show_flow_run for the current generation")
		}
		return flowErrorResult(ctx, err)
	}
	if out.Written == nil {
		out.Written = []string{}
	}
	return jsonResult(out)
}
