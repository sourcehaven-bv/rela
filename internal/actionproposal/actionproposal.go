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
