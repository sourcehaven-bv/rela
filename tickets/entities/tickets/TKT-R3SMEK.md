---
id: TKT-R3SMEK
type: ticket
title: 'appbuild: extract ScheduledMailer (scheduler behaviour off the wiring facade), then make Services'' exported-method directive a documented exception'
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
status: ready
---

Sub-ticket of [[TKT-N0IKN9]] — `appbuild.Services` (30 exported, directive at
`internal/appbuild/appbuild.go:100`).

## Diagnosis: three groups, one of which does not belong

1. **21 plain accessors** (`FS`, `Paths`, `Meta`, `Store`, … `CalDAVAliases`),
each returning one unexported field. Their purpose is to be the structural seam
a consumer-side interface binds against — `dataentry.ResolverServices`
(affordances_stub.go:104) declares exactly `ACLDeclarative()/Meta()/Store()` and
takes `svc`. That is the project's central rule working. **Do not group these
into `Read()`/`Write()` sub-bundles**: a method set on a RETURNED struct cannot
be satisfied by `*Services`, so every such consumer interface breaks.
(`cli.readServices` is a precedent for consumers building bundles FROM Services
— cli_wiring.go:126-135 — not for restructuring the producer.) Exported fields
instead would trade the method cap for the field cap (26 fields, line 20) and
destroy the `Close` invariants (`Close()` nils
`mailStop`/`searchCloser`/`jobQueue`, :1779-1795).
2. **4 constructed bundles** (`LuaReadDeps`, `LuaWriteDeps`,
`ScheduledLuaWriteDeps`, `GatedReads`) — already the sub-bundle pattern.
3. **4 scheduler behaviours** — `RunScheduledTemplate` (scheduled_mail.go:20),
`ValidateScheduledMailRecipients` (scheduled_mail_validate.go:19),
`ScheduledForEachEntities` / `ScheduledForEachPrincipal`
(scheduler_foreach.go:14, :59). A wiring facade that constructs collaborators
should not also render and send mail. They sit on Services only so `*Services`
structurally satisfies the scheduler's narrow interfaces (foreach.go:16-25,
scheduler.go:90-94) — the same shape as Metamodel growing a migration provider.

## The extraction

```go
// ScheduledMailer is the scheduler-facing behaviour surface. A separate type
// because Services is a WIRING facade — it constructs collaborators, it does
// not render and send mail. Valid only while svc is open.
type ScheduledMailer struct{ svc *Services }
func (s *Services) Scheduled() ScheduledMailer { return ScheduledMailer{svc: s} } // fresh value, NOT a cached field (max-fields)
func (m ScheduledMailer) RunScheduledTemplate(ctx, name, recipientID string) error
func (m ScheduledMailer) ValidateScheduledMailRecipients(ctx) error
func (m ScheduledMailer) ScheduledForEachEntities(…) ([]string, int, error)
func (m ScheduledMailer) ScheduledForEachPrincipal(ctx, entityID string) (string, error)
func (m ScheduledMailer) ScheduledLuaWriteDeps(…)  // forward, so scheduler.WorkspaceProvider binds ONE value
```

Bodies move verbatim (they reach only `svc.ScheduledLuaWriteDeps()`, `svc.mail`,
`svc.meta`). Call sites: scheduler wiring passes `svc.Scheduled()`;
`internal/cli/validate.go:85` becomes
`mailSvc.Scheduled().ValidateScheduledMailRecipients(ctx)`.

## Then: stop letting the cap distort the type

`fieldRedactor` is documented at appbuild.go:151-160 as "a FIELD, not an
accessor, on purpose — Services sits at its ceiling". That is the cap causing a
design wart. Restore `FieldRedactor()` as a proper accessor and delete the
rationale. `ACLPolicy()` has zero production callers but `sharedbase_test.go:75`
pins the shared-policy invariant through it — keep, note it.

Net 30 − 4 + 1 + 1 = **28**, still over 20. Convert the directive from a ratchet
target to a documented exception in the same words CLAUDE.md uses for store
implementations: each exported method returns exactly one constructed
collaborator and exists to be a consumer-side seam; the count tracks how many
collaborators the application has, not internal sprawl.

## Invariants (sharedbase_test.go:66-115)

- Assembly never mutates `meta` / `aclPolicy`; `Close` tears down only
per-assembly state. `ScheduledMailer` holds `*Services` and adds no resource, so
both hold — pin with a one-line test that `Scheduled()` returns a value with no
shared state.

## Follow-up worth its own ticket (not here)

`cmd/rela-server/main.go:456-460` calls `dataentry.NewApp` with 12 positional
arguments — a `dataentry.Deps` struct (the `lua.ReadDeps` pattern) is the
capability-bundle rule applied to the biggest consumer. A `dataentry` problem,
not a `Services` one.
