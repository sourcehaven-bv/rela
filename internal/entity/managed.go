package entity

import "strings"

// ManagedPrefix starts every type, relation and property name that rela
// manages itself. A schema may not declare such a name, and acl.yaml may not
// grant one, so a rela-managed name never collides with an operator's and
// only rela decides who writes it.
//
// It lives here, below metamodel and acl, so the ACL can recognize a managed
// name without importing the schema package.
const ManagedPrefix = "_"

// The rela-managed names of flow runs (FEAT-J0O10QI). A run is an entity of
// [FlowRunType]; [CarriesRelation] points from the run to the entity it
// carries, its subject; [ProducedRelation] points from the run to an entity
// one of its steps created.
const (
	FlowRunType      = "_flow-run"
	CarriesRelation  = "_carries"
	ProducedRelation = "_produced"
)

// IsManagedName reports whether name, or its local part after a namespace
// separator (`isms._run`), starts with [ManagedPrefix].
func IsManagedName(name string) bool {
	if strings.HasPrefix(name, ManagedPrefix) {
		return true
	}
	_, local, ok := strings.Cut(name, ".")
	return ok && strings.HasPrefix(local, ManagedPrefix)
}
