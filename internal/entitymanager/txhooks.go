package entitymanager

// TxHooks are the optional collaborators that run inside the store
// transaction of a write, where no post-commit hook can act. Each is nil
// when nothing needs it.
type TxHooks struct {
	// RenameGuard refuses a rename that state kept outside the graph under
	// the old id forbids (see [RenameGuard]).
	//
	// Nil: accepted — no such state, as a build without flows has.
	RenameGuard RenameGuard

	// CreateHook is told of each new entity inside the create's store
	// transaction (see [CreateHook]).
	//
	// Nil: accepted — nothing watches creates, as a build without a flow
	// that starts on create has.
	CreateHook CreateHook

	// UpdateHook is told of each update inside the update's store
	// transaction (see [UpdateHook]).
	//
	// Nil: accepted — nothing watches updates, as a build without flows
	// has.
	UpdateHook UpdateHook
}
