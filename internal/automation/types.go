// Package automation provides a trigger-action engine for entity lifecycle events.
// It enables automatic property updates, validation warnings, and relation creation
// when entities change.
package automation

import (
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Automation defines a trigger-action rule (internal representation).
type Automation struct {
	Name        string
	Description string
	On          Trigger
	Do          []Action
	Validate    []Validation
}

// Trigger specifies conditions that activate an automation.
type Trigger struct {
	Entity          []string
	Property        string
	Becomes         string
	From            string
	Created         bool
	RelationCreated string
	RelationRemoved string
	When            []*filter.Filter // Property conditions that must all match
}

// Action specifies an operation to perform.
type Action struct {
	Set            string
	Value          string
	CreateRelation *CreateRelationAction
	CreateEntity   *CreateEntityAction
	Lua            string // Inline Lua code to execute
	LuaFile        string // Path to Lua script file in scripts/ directory
}

// CreateRelationAction specifies parameters for creating a relation.
type CreateRelationAction struct {
	Relation string
	To       string
}

// CreateEntityAction specifies parameters for creating a new entity.
type CreateEntityAction struct {
	Type       string            // Entity type to create (e.g., "planning-checklist")
	Template   string            // Optional: template variant name, supports interpolation (e.g., "{{new.kind}}" loads <type>--<kind>.md)
	Properties map[string]string // Properties to set (values support interpolation)
	Relation   string            // Optional: relation type FROM triggering entity TO created entity
	IfExists   string            // Behavior when relation exists: skip (default), error, replace
}

// IfExists constants for CreateEntityAction behavior.
const (
	IfExistsSkip    = "skip"    // Skip creation if relation already exists (default)
	IfExistsError   = "error"   // Return error if relation already exists
	IfExistsReplace = "replace" // Delete existing and create new
)

// Validation specifies a condition to check.
type Validation struct {
	Check    string
	Severity string
	Message  string
}

// GetSeverity returns the severity, defaulting to "warning".
func (v *Validation) GetSeverity() string {
	if v.Severity == "" {
		return "warning"
	}
	return v.Severity
}

// Event represents a change that occurred to an entity or relation.
type Event struct {
	// Type is the type of event.
	Type EventType

	// Entity is the affected entity.
	Entity *model.Entity

	// OldEntity is the previous state (nil for Created events).
	OldEntity *model.Entity

	// Relation is the affected relation (for RelationCreated/RelationRemoved events).
	Relation *model.Relation
}

// EventType identifies the kind of change.
type EventType int

const (
	// EventEntityCreated fires when a new entity is created.
	EventEntityCreated EventType = iota

	// EventEntityUpdated fires when an entity's properties change.
	EventEntityUpdated

	// EventRelationCreated fires when a new relation is created.
	EventRelationCreated

	// EventRelationRemoved fires when a relation is removed.
	EventRelationRemoved
)

// EntityToCreate specifies an entity to be created by automation.
type EntityToCreate struct {
	Type                string                 // Entity type to create
	Template            string                 // Optional: template variant name
	Properties          map[string]interface{} // Properties for the new entity
	RelationFromTrigger string                 // Optional: relation type from triggering entity
	IfExists            string                 // Behavior when relation exists: skip (default), error, replace
}

// LuaToExecute specifies Lua code to be executed by the workspace layer.
// Either Code or FilePath is set, not both.
type LuaToExecute struct {
	Code     string // Inline Lua code (safe values already interpolated)
	FilePath string // Path to script file in scripts/ directory
}

// Result represents the outcome of running automations.
type Result struct {
	// PropertiesSet contains properties that were automatically set.
	PropertiesSet map[string]string

	// RelationsToCreate contains relations that should be created.
	RelationsToCreate []*model.Relation

	// EntitiesToCreate contains entities that should be created.
	EntitiesToCreate []EntityToCreate

	// LuaToExecute contains Lua scripts to be executed by the workspace layer.
	LuaToExecute []LuaToExecute

	// Warnings contains validation warnings (allow save, show message).
	Warnings []string

	// Errors contains validation errors (should block save in strict mode).
	Errors []string
}

// HasWarnings returns true if there are any warnings.
func (r *Result) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// HasErrors returns true if there are any errors.
func (r *Result) HasErrors() bool {
	return len(r.Errors) > 0
}

// AllMessages returns all warnings and errors combined.
func (r *Result) AllMessages() []string {
	msgs := make([]string, 0, len(r.Warnings)+len(r.Errors))
	msgs = append(msgs, r.Errors...)
	msgs = append(msgs, r.Warnings...)
	return msgs
}
