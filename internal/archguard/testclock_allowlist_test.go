package archguard

// wallClockAllowlist pins every time.Now in the write-path packages (see
// wallClockReads), per repo-relative file. It may only shrink. Each entry
// says why that read stays on the wall clock rather than the world's clock.
var wallClockAllowlist = map[string]allowed{
	"internal/automation/template.go": {3, "the default clock: DefaultTemplateVars and the " +
		"Interpolate fallback when a caller passes no TemplateVars.Now; the engine sets Now " +
		"from its WithClock option"},
	"internal/entitymanager/audithook.go": {4, "audit record stamps: audit records when a write " +
		"really happened, so a pinned clock must not backdate it"},
	"internal/entitymanager/versiontags.go":     {1, "version-history stamp; stays on the wall clock like audit"},
	"internal/entitymanager/cascadehost.go":     {1, "audit record stamp of a cascade write"},
	"internal/entitymanager/conversion_flow.go": {1, "audit record stamp of a conversion"},
	"internal/entitymanager/manager.go": {4, "three audit record stamps, and the default of " +
		"Deps.Clock when the wiring site sets none"},
	"internal/computed/computed.go": {1, "Compile is CompileWithClock on the wall clock; " +
		"appbuild calls CompileWithClock"},
	"internal/lua/cache.go": {1, "rela.cache TTL expiry: a cache lifetime is real elapsed time, " +
		"not world time"},
	"internal/lua/runtime.go":             {1, "the default clock when neither a seam nor WithClock sets one"},
	"internal/lua/http.go":                {1, "Retry-After is relative to the real server's clock"},
	"internal/store/memstore/memstore.go": {1, "the default of the WithClock option"},
	"internal/lease/service.go":           {1, "SystemClock, the production lease.Clock; appbuild passes the world clock"},
	"internal/appbuild/scheduled_mail.go": {1, "servicesNow's fallback for Services built without a SharedBase"},
	"internal/predicatefns/evaluator.go": {1, "NewEvaluator is NewEvaluatorWithClock on the wall clock; " +
		"world wiring calls NewEvaluatorWithClock"},
}
