package audit

import (
	"context"
	"strings"
)

// FlowStep names the flow step a write runs in: the flow's name in
// flows.yaml, the run's entity id and the step. A write made under
// [WithFlowStep] records all three (TKT-BC5G8RP).
type FlowStep struct {
	Flow string
	Run  string
	Step string
	// By is the principal who started or cancelled the run by hand; empty
	// for a step the runtime takes on its own.
	By string
}

// flowStepKey is the context key of the current [FlowStep].
type flowStepKey struct{}

// WithFlowStep returns a derived context carrying s. The flow engine
// stamps it around each step; an automation the step's writes trigger
// runs in the same context, so its records carry the step too. A job the
// step queues does not: the queue carries no context values.
func WithFlowStep(ctx context.Context, s FlowStep) context.Context {
	return context.WithValue(ctx, flowStepKey{}, s)
}

// FlowStepFrom returns the [FlowStep] carried by ctx, and whether one was
// stamped.
func FlowStepFrom(ctx context.Context) (FlowStep, bool) {
	s, ok := ctx.Value(flowStepKey{}).(FlowStep)
	return s, ok
}

// Stamp copies the [FlowStep] carried by ctx, if any, onto rec. The
// entity manager stamps every entity and relation write record with it.
func Stamp(ctx context.Context, rec Record) Record {
	if s, ok := FlowStepFrom(ctx); ok {
		rec.Flow, rec.Run, rec.Step, rec.By = s.Flow, s.Run, s.Step, s.By
	}
	return rec
}

// Source values. A record's source says what kind of actor produced it,
// next to the principal that answers for it.
const (
	SourceFlow       = "flow"
	SourceAutomation = "automation"
	SourceSchedule   = "schedule"
	SourceCascade    = "cascade"
	SourceRestore    = "restore"
	SourceRenumber   = "renumber"
	SourceRename     = "rename"
	SourceConvert    = "convert"
	SourceChannel    = "channel"
)

// triggerSources maps the kind of a triggered-by label (the part before
// the first ':' or ';') to its source. It is closed: a label of another
// kind falls back to the principal's tool, so the vocabulary of `source`
// changes only here.
var triggerSources = map[string]string{
	"automation":     SourceAutomation,
	"automation-job": SourceAutomation,
	"schedule":       SourceSchedule,
	"cascade":        SourceCascade,
	"restore-entity": SourceRestore,
	"renumber":       SourceRenumber,
	"rename":         SourceRename,
	"rename-entity":  SourceRename,
	"convert-entity": SourceConvert,
	// The channel engine: promotes, outputs, reminders, timers, trigger
	// rules and automatic operations.
	"channel":  SourceChannel,
	"session":  SourceChannel,
	"reminder": SourceChannel,
	"timer":    SourceChannel,
	"trigger":  SourceChannel,
	"auto":     SourceChannel,
}

// SourceOf derives rec's source: [SourceFlow] for a write made in a flow
// step, else the kind of its triggered-by label, else the principal's
// tool (`cli`, `mcp`, `data-entry`, ...). The flow step wins over the
// label: an automation a step's write triggers records `flow`, and its
// triggered_by still names the automation. The sinks fill it in for every
// record that does not carry one.
func SourceOf(rec Record) string {
	if rec.Flow != "" {
		return SourceFlow
	}
	if rec.TriggeredBy != "" {
		kind, _, _ := strings.Cut(rec.TriggeredBy, ";")
		kind, _, _ = strings.Cut(kind, ":")
		if s, ok := triggerSources[kind]; ok {
			return s
		}
	}
	return rec.Principal.Tool
}

// withSource fills in rec.Source when it is empty.
func withSource(rec Record) Record {
	if rec.Source == "" {
		rec.Source = SourceOf(rec)
	}
	return rec
}
