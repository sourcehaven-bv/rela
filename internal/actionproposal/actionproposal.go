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
	// Model is the model that answered, as the provider reports it.
	Model string `json:"model,omitempty"`
}

// Empty is a proposal of no changes for entity. Its lists encode as [].
func Empty(entity string) *Proposal {
	return &Proposal{Entity: entity, Set: []Value{}, Relate: []Relation{}}
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
