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
	Entity string `json:"entity"`
	// Title is the entity's display title as the caller may see it. A batch
	// run (TKT-I2SRNWR) sets it so the review dialog can name each entity.
	Title  string     `json:"title,omitempty"`
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

// Batch is the reply of a batch run of an `llm:` action (TKT-I2SRNWR): one
// proposal per entity the model answered for, and the entities it could not.
type Batch struct {
	// Proposals is never nil, so a run with no usable row still encodes as
	// a batch rather than as an empty response.
	Proposals []*Proposal `json:"proposals"`
	// Failed lists the entities whose part of the reply was unusable.
	Failed []Failure `json:"failed"`
	// Skipped counts the addresses the caller sent that were not run: not
	// readable, not offered the action, or with no output the caller may
	// change. They are counted, not named, so the count says nothing the
	// caller did not already know.
	Skipped int `json:"skipped"`
}

// NewBatch is an empty batch reply. Its lists encode as [].
func NewBatch() *Batch {
	return &Batch{Proposals: []*Proposal{}, Failed: []Failure{}}
}

// Failure is one entity of a batch run whose part of the reply could not
// be used.
type Failure struct {
	Entity string `json:"entity"`
	Title  string `json:"title,omitempty"`
	Reason string `json:"reason"`
}
