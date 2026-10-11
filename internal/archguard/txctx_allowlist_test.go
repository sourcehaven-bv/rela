package archguard

// txCtxAllowlist pins the Tx callbacks that do not mark their ctx with
// store.ContextInTx (see unmarkedTxCallbacks). It may only shrink.
var txCtxAllowlist = map[string]allowed{
	"internal/perfseed/load.go": {1, "the write callback captures the caller's ctx and cannot take " +
		"the marked one; perf seeding writes rows only and never tags a version"},
	"internal/store/sqlitestore/entity.go": {4, "the store's own write methods opening their " +
		"transaction; the callback runs store internals, not caller code that could tag"},
	"internal/store/sqlitestore/rename.go": {1, "the store's own rename opening its transaction " +
		"through a named callback; store internals only"},
	"internal/store/sqlitestore/softdelete.go": {1, "the store's own soft-delete helper opening its " +
		"transaction; store internals only"},
}
