// Package actionproposal is the reply of an `llm:` action (TKT-4USH1HO):
// changes to one entity that the action proposes but does not write.
//
// It is a leaf so that every transport (the data-entry API, MCP and the
// script layer between them) carries the same type.
package actionproposal

import "encoding/json"

// Proposal is the changes an `llm:` action proposes for one entity. The
// caller applies the ones they accept through the ordinary write APIs.
type Proposal struct {
	// Entity is the address of the entity the changes are for.
	Entity string     `json:"entity"`
	Set    []Value    `json:"set"`
	Relate []Relation `json:"relate"`
	// Create is new entities to create, each related to Entity.
	Create []NewEntity `json:"create"`
	// Model is the model that answered, as the provider reports it.
	Model string `json:"model,omitempty"`
}

// Empty is a proposal of no changes for entity. Its lists encode as [].
func Empty(entity string) *Proposal {
	return &Proposal{Entity: entity, Set: []Value{}, Relate: []Relation{}, Create: []NewEntity{}}
}

// Value is one proposed property value, as JSON. Current is the value the
// caller could see when the action ran, or null when it was unset.
type Value struct {
	Property string          `json:"property"`
	Current  json.RawMessage `json:"current"`
	Proposed json.RawMessage `json:"proposed"`
}

// Relation is one proposed outgoing relation from the entity to To.
type Relation struct {
	Relation string `json:"relation"`
	To       string `json:"to"`
	ToType   string `json:"to_type"`
	ToTitle  string `json:"to_title"`
}

// NewEntity is one proposed new entity of Type, related to the proposal's
// entity through Relation: from the entity to the new one when Outgoing,
// else from the new one to the entity. Its Properties have a null Current.
type NewEntity struct {
	Type       string  `json:"type"`
	Relation   string  `json:"relation"`
	Outgoing   bool    `json:"outgoing"`
	Properties []Value `json:"properties"`
}

// IsEmpty reports whether p proposes no change.
func (p *Proposal) IsEmpty() bool {
	return p == nil || len(p.Set) == 0 && len(p.Relate) == 0 && len(p.Create) == 0
}
