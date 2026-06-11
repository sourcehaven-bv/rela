---
id: RR-G4I74
type: review-response
title: 'DatabaseURL dual path: Config field + WithDatabaseURL option that overrides it'
finding: 'go-architect (S2): the DSN has two entry points that interact confusingly — appbuild.Config.DatabaseURL (a backend-specific field on the shared Config struct, ignored by 2 of 3 builds) AND appbuild.WithDatabaseURL option (which overrides the field in prepare()). Three touch points: Config field, option, and the override in prepare. The architect suggests routing the DSN through the option ONLY (drop the Config field), so backend-specific config doesn''t live on the shared struct. Counterpoint (per the user''s Stage-B design decision): Config was deliberately chosen as the home for future per-scenario config (audit/ACL knobs), and Discover reads RELA_DATABASE_URL into the field. The dual path exists because Discover builds Config internally yet entry-point flags must still override the env.'
severity: minor
reason: 'Deliberate Stage-B design decision (see RR-E7WNC + planning discussion): appbuild.Config is the chosen home for per-scenario config, anticipating future audit/ACL knobs. The dual path (Config.DatabaseURL + WithDatabaseURL override) is the minimal cost of Discover() building Config from env internally while entry-point flags, parsed afterward, must still override. Precedence is documented in WithDatabaseURL and prepare(). Not a correctness issue; keeping the Config field preserves the symmetry the user explicitly chose. Revisit only if a cleaner config story emerges across backends.'
status: wont-fix
---

## Assessment

This was a deliberate Stage-B decision (Config struct as the per-scenario config
home; see RR-E7WNC and the planning discussion). The dual path is the cost of:
Discover() building Config from env internally, while flags (which kong/flag
parse after) must override.

Options:
- **Keep as-is** (status quo): Config.DatabaseURL is the canonical field; the
option is purely the flag-override channel. Document the precedence clearly
(already commented in WithDatabaseURL + prepare).
- **Option-only:** drop Config.DatabaseURL; Discover reads env and passes
WithDatabaseURL(os.Getenv(...)). Removes the field from the shared struct but
loses the "Config is where backend config lives" symmetry the user chose.

Lower severity — not a correctness issue. Defer to the user's Stage-B preference
(Config struct) unless they want to revisit. Recommend: keep Config.DatabaseURL,
remove the option-override duplication by having entry points set the field
directly... but Discover builds Config, so the option is still needed for the
flag. Net: status quo is the pragmatic choice; mark wont-fix unless user prefers
option-only.
