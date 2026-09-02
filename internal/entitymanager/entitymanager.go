// Package entitymanager defines the entity-manager service — the "human intent"
// write path that runs automations, validation, and policy concerns (ACL,
// audit logging).
//
// This sits above the Store: reads and writes still go through a store.Store
// backend, but the manager adds workflow concerns that shouldn't live in raw
// storage. Consumers use the manager; the store stays focused on CRUD.
//
// Not all consumers need a manager. Importers, bulk sync, and formatters
// bypass automations and talk to the store directly.
//
// # No interface here
//
// [Manager] is the whole API, and it is a concrete type on purpose. This
// package deliberately does NOT declare a wide `EntityManager` interface
// beside it: that was a producer-side interface, which CLAUDE.md's first rule
// forbids, and every consumer bound to all nine methods while using between
// one and seven of them. It was removed in TKT-IVSJV6.
//
// Each consumer now declares its own narrow interface at its call site, naming
// only what it invokes — [github.com/Sourcehaven-BV/rela/internal/lua.Mutator]
// (6 methods), attachment.EntityUpdater (1), mcp.EntityWriter (6), and the
// unexported ones in internal/cli and internal/dataentry. *Manager satisfies
// each structurally, so adding a method here breaks nothing and a consumer
// reaching for a new capability has to say so in its own interface.
//
// A composition root that DISTRIBUTES the manager to sub-handlers (dataentry's
// NewApp) takes the concrete *Manager rather than a narrowed value, so each
// handler narrows at its own field. Otherwise the root would have to hold the
// union of every child's needs — the same "wide because it is a distributor"
// shape, one level down.
//
// Read operations are intentionally absent from all of them — consumers read
// directly from store.Store.
package entitymanager
