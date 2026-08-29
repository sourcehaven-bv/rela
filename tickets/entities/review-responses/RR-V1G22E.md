---
id: RR-V1G22E
type: review-response
title: Warning fires per script execution and twice per mail send, not once
finding: |-
    The plan's mitigation for warning fatigue says the warning is emitted "once per load, not per key" — but a load is not a rare event. secrets.Load is called from lua.LoadContextOptions (internal/lua/context.go:35), which runs inside script.NewWriterRuntime — constructed per document render (internal/script/list_document.go:85), per action (internal/script/action.go:68), and per executor run (internal/script/executor.go:201). On the mail side, Config.resolvePassword calls Load and is invoked twice per send: once via hasPassword (internal/mail/smtp.go:78) and again directly (smtp.go:111).

    A data-entry server rendering documents would emit this warning on every page render, which is log spam severe enough that operators will filter the logger — destroying the value of the warning and any other warning from the same package. The plan's stated mitigation does not address this because it misidentifies the call frequency.
severity: significant
resolution: Added a package-level sync.Map (warnedPaths) keyed by resolved path; warnIfPermissive uses LoadOrStore so a given file warns once per process. Keyed per path rather than a single sync.Once so a multi-project process still warns for each project. Pinned by TestLoad_WarnsOncePerPath (3 loads -> 1 warning) and TestLoad_WarnsPerProjectNotGlobally (2 projects -> 2 warnings).
status: addressed
---

## Fix

De-duplicate the warning per resolved path for the process lifetime, using a
`sync.Map` (or a mutex-guarded `map[string]struct{}`) keyed by the absolute
path. First observation of a permissive file warns; subsequent loads of the same
path stay silent.

Per-path rather than a single `sync.Once` because a process can legitimately
serve multiple projects (`appbuild.SharedBase` assembles one `Services` per
tenant), and a global once would silence the warning for every project after the
first — the exact failure the ceiling-guard reasoning in CLAUDE.md warns about,
where a shared object leaks state across tenants.

This keeps the warning actionable (it still fires once per misconfigured file)
without coupling it to render frequency.
