---
id: RR-FSVCTX
type: review-response
title: FSView hardcoded context.Background, severing the cancellation seam
severity: significant
status: addressed
finding: 'Loader.Load and Loader.List take a context; FSView discarded it and passed context.Background() at every call site. Harmless for FSLoader, but the SQLite loader this feature exists to enable can genuinely block — waiting on a locked database file — and would then be uncancellable. The irony is pointed: internal/datamigration binds a `lua:` step''s VM to the run context precisely so a runaway migration is interruptible by Ctrl-C, and then read that step''s script through an FS that ignored ctx entirely.'
resolution: 'Added FSView.WithContext, since fs.FS has no context in any of its method signatures and a ctx therefore has to live on the value. The `lua:` call site in migrate_data.go binds the runner''s ctx, which is the case that motivated it. loadDataMigrations and newConfigFS now take a ctx as their first parameter so the linter''s contextcheck is satisfied rather than suppressed; only the stored field carries a containedctx suppression, with the reason at the field. Nil ctx is rejected rather than silently replaced.'
---
