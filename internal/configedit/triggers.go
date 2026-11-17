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
}

// TriggerProblem is why one trigger is refused.
type TriggerProblem struct {
	Trigger string
	Message string
}

// triggerProblems asks the TriggerChecker about a save that changes
// flows.yaml or schema.yaml. A schema change can widen what an unchanged
// trigger fires on, so it is judged too. The answer is a refusal when the
// checker cannot judge the edit.
func (s *Service) triggerProblems(ctx context.Context, cur *current, p *prepared) []Problem {
	before := cur.flowsBytes()
	if bytes.Equal(p.schema, cur.schema) && bytes.Equal(p.flows, before) && (p.flows == nil) == (before == nil) {
		return nil
	}
	found, err := s.deps.Triggers.CheckTriggers(ctx, TriggerEdit{
		ACL:          cur.acl,
		SchemaBefore: cur.schema, SchemaAfter: p.schema,
		FlowsBefore: before, FlowsAfter: p.flows,
	})
	if err != nil {
		return []Problem{{
			Code: "acl_trigger", File: FlowsFile, Path: "triggers",
			Message: fmt.Sprintf("The automations could not be checked against acl.yaml (%v), "+
				"so this change is made in flows.yaml directly.", err),
		}}
	}
	out := make([]Problem, 0, len(found))
	for _, f := range found {
		out = append(out, Problem{
			Code: "acl_trigger", File: FlowsFile, Path: "triggers." + f.Trigger, Value: f.Trigger,
			Message: f.Message,
		})
	}
	return out
}
