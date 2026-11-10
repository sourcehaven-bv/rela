package metamodel

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// FlowRunIDPrefix is the id prefix of a [entity.FlowRunType] entity.
const FlowRunIDPrefix = "RUN-"

// The properties of a [entity.FlowRunType] entity. Every value is engine
// state or comes from flows.yaml, never from another entity, so a reader of
// the run learns nothing its subject's readers could not already see
// (TKT-BC5G8RP).
const (
	FlowRunPropFlow      = "flow"
	FlowRunPropVersion   = "version"
	FlowRunPropStatus    = "status"
	FlowRunPropState     = "state"
	FlowRunPropStarted   = "started"
	FlowRunPropCompleted = "completed"
	FlowRunPropDeadline  = "deadline"
)

// FlowRunStatuses are the values of the status property: the statuses of
// [flow.Status], in its declaration order.
var FlowRunStatuses = []string{"working", "input_required", "completed", "cancelled", "failed"}

// WithFlowRuns returns a copy of m that also declares the rela-managed flow
// run type and its two relations (FEAT-J0O10QI). m must have passed
// validation, and is not changed: it may be a published snapshot.
//
// `_carries` and `_produced` target every faceless operator type. A flow
// refuses a faced subject and a faced create type, and a run's read gate
// evaluates its subject at the reader's world, which only a faceless type
// reads the same in every world. The targets are computed before the
// managed type is added, so the run type is never a target of its own
// relations.
//
// Nil: m is rejected.
func WithFlowRuns(m *Metamodel) (*Metamodel, error) {
	if m == nil {
		return nil, errors.New("metamodel: WithFlowRuns requires a metamodel")
	}
	if m.flowRuns {
		return nil, errors.New("metamodel: WithFlowRuns: the flow run type is already declared")
	}
	targets := flowRunTargets(m)
	if len(targets) == 0 {
		return nil, errors.New("metamodel: WithFlowRuns: no faceless entity type for a run to carry")
	}
	for name, def := range m.Entities {
		if slices.ContainsFunc(def.GetIDPrefixes(), sameIDPrefix) {
			return nil, fmt.Errorf("metamodel: WithFlowRuns: entity %q already uses the id prefix %q of flow runs",
				name, FlowRunIDPrefix)
		}
	}

	c := cloneForValidate(m)
	if c.Relations == nil {
		// A schema without `relations:` parses to a nil map.
		c.Relations = map[string]RelationDef{}
	}
	c.Entities[entity.FlowRunType] = flowRunEntityDef()
	c.Relations[entity.CarriesRelation] = RelationDef{
		Label:       "Carries",
		Description: "The entity a flow run carries: its subject.",
		From:        []string{entity.FlowRunType},
		To:          targets,
	}
	c.Relations[entity.ProducedRelation] = RelationDef{
		Label:       "Produced",
		Description: "An entity a step of the flow run created.",
		From:        []string{entity.FlowRunType},
		To:          slices.Clone(targets),
	}
	c.flowRuns = true
	if err := validate(c); err != nil {
		return nil, fmt.Errorf("metamodel: WithFlowRuns: %w", err)
	}
	return c, nil
}

// cloneForValidate copies m deeply enough that validate may run on the
// copy while other goroutines read m: validate writes the Entities,
// Relations and Types maps, the compiled patterns of each type's
// validations, and the comment policy.
func cloneForValidate(m *Metamodel) *Metamodel {
	c := *m
	c.Entities = maps.Clone(m.Entities)
	c.Relations = maps.Clone(m.Relations)
	c.Transforms = maps.Clone(m.Transforms)
	c.Types = make(map[string]CustomType, len(m.Types))
	for name, t := range m.Types {
		t.Validations = slices.Clone(t.Validations)
		c.Types[name] = t
	}
	if m.Comments != nil {
		cc := *m.Comments
		cc.On = slices.Clone(m.Comments.On)
		c.Comments = &cc
	}
	return &c
}

// HasFlowRuns reports whether m declares the flow run type through
// [WithFlowRuns].
func (m *Metamodel) HasFlowRuns() bool { return m != nil && m.flowRuns }

// flowRunTargets returns the faceless operator types of m, sorted.
func flowRunTargets(m *Metamodel) []string {
	var out []string
	for _, name := range sortedKeys(m.Entities) {
		if IsReservedName(name) || len(m.Entities[name].Faces) > 0 {
			continue
		}
		out = append(out, name)
	}
	return out
}

// sameIDPrefix reports whether p mints ids that collide with
// [FlowRunIDPrefix]. Ids are matched without regard to case.
func sameIDPrefix(p string) bool {
	return strings.EqualFold(strings.TrimSuffix(p, "-"), strings.TrimSuffix(FlowRunIDPrefix, "-"))
}

// isManagedDecl reports whether name is one of the declarations
// [WithFlowRuns] added to m. Only that function sets the flag, so an
// operator's schema cannot claim the exemption.
func (m *Metamodel) isManagedDecl(name string) bool {
	if !m.flowRuns {
		return false
	}
	return name == entity.FlowRunType || name == entity.CarriesRelation || name == entity.ProducedRelation
}

func flowRunEntityDef() EntityDef {
	dt := func(label, desc string) PropertyDef {
		return PropertyDef{Label: label, Type: PropertyTypeDatetime, Description: desc}
	}
	return EntityDef{
		Label:       "Flow run",
		LabelPlural: "Flow runs",
		Description: "One run of a flow from flows.yaml. rela writes it; nobody else can.",
		Plural:      "_flow-runs",
		IDPrefix:    FlowRunIDPrefix,
		Properties: map[string]PropertyDef{
			FlowRunPropFlow: {
				Label: "Flow", Type: PropertyTypeString, Required: true,
				Description: "The name of the flow in flows.yaml.",
			},
			FlowRunPropVersion: {
				Label: "Version", Type: PropertyTypeString, Required: true,
				Description: "The content hash of the flow definition the run follows.",
			},
			FlowRunPropStatus: {
				Label: "Status", Type: PropertyTypeEnum, Required: true, Values: slices.Clone(FlowRunStatuses),
				Description: "What the run's active state means to someone waiting on it.",
			},
			FlowRunPropState: {
				Label: "State", Type: PropertyTypeString,
				Description: "The name of the run's active state.",
			},
			FlowRunPropStarted:   dt("Started", "When the run started."),
			FlowRunPropCompleted: dt("Completed", "When the run reached a final state."),
			FlowRunPropDeadline:  dt("Deadline", "When the run is due, from the flow's deadline."),
		},
		PropertyOrder: []string{
			FlowRunPropFlow, FlowRunPropVersion, FlowRunPropStatus, FlowRunPropState,
			FlowRunPropStarted, FlowRunPropCompleted, FlowRunPropDeadline,
		},
		DisplayProperty: FlowRunPropFlow,
	}
}
