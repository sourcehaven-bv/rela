package configedit

import (
	"bytes"
	"context"
	"fmt"
)

// TriggerChecker judges a change to the flows.yaml triggers against the
// access policy in acl.yaml. A trigger output runs as a cascade of the write
// that fired it, and the entity manager checks neither the row ACL nor the
// field gate for it. So a trigger edited in the app may not read what the
// policy hides from some reader, write what the policy decides access by,
// or write what the editor could not write directly.
//
// It is declared here, where it is used: this package may not import the
// acl package, so the server supplies the implementation at wiring.
type TriggerChecker interface {
	// CheckTriggers returns one problem for each refused trigger. An error
	// means the edit could not be judged; the save is then refused.
	CheckTriggers(ctx context.Context, edit TriggerEdit) ([]TriggerProblem, error)
}

// TriggerEdit is the change to judge: the files as read and as the save
// would write them. Flows fields are nil when flows.yaml is absent.
type TriggerEdit struct {
	ACL                       []byte
	SchemaBefore, SchemaAfter []byte
	FlowsBefore, FlowsAfter   []byte
	// DataEntryBefore and DataEntryAfter are data-entry.yaml, whose forms a
	// flow step edits: a trigger may not write what a step form edits.
	DataEntryBefore, DataEntryAfter []byte
}

// TriggerProblem is why one trigger, or the change to one step form, is
// refused.
type TriggerProblem struct {
	Trigger string
	// Form names the data-entry.yaml form whose change is refused. Trigger
	// is then empty.
	Form    string
	Message string
}

// triggerProblems asks the TriggerChecker about a save that changes
// flows.yaml or schema.yaml, or data-entry.yaml while flows.yaml exists. A
// schema change can widen what an unchanged trigger fires on, and a form
// change what a flow step locks, so they are judged too. The answer is a
// refusal when the checker cannot judge the edit.
func (s *Service) triggerProblems(ctx context.Context, cur *current, p *prepared) []Problem {
	before := cur.flowsBytes()
	if bytes.Equal(p.schema, cur.schema) && bytes.Equal(p.flows, before) && (p.flows == nil) == (before == nil) &&
		(before == nil || bytes.Equal(p.dataEntry, cur.dataEntry)) {

		return nil
	}
	found, err := s.deps.Triggers.CheckTriggers(ctx, TriggerEdit{
		ACL:          cur.acl,
		SchemaBefore: cur.schema, SchemaAfter: p.schema,
		FlowsBefore: before, FlowsAfter: p.flows,
		DataEntryBefore: cur.dataEntry, DataEntryAfter: p.dataEntry,
	})
	if err != nil {
		return []Problem{{
			Code: "acl_trigger", File: FlowsFile, Path: triggersKey,
			Message: fmt.Sprintf("The automations could not be checked against acl.yaml (%v), "+
				"so this change is made in flows.yaml directly.", err),
		}}
	}
	out := make([]Problem, 0, len(found))
	for _, f := range found {
		if f.Form != "" {
			out = append(out, Problem{
				Code: "step_form", File: DataEntryFile, Path: "forms." + f.Form, Value: f.Form, Message: f.Message,
			})
			continue
		}
		out = append(out, Problem{
			Code: "acl_trigger", File: FlowsFile, Path: triggersKey + "." + f.Trigger, Value: f.Trigger,
			Message: f.Message,
		})
	}
	return out
}
