---
id: DOCS-YEI6IN
type: docs-checklist
title: 'Documentation: Declarative webhook routes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols have godoc
- [x] Non-obvious decisions explained at the call site

The comments that matter carry the *why*, not the what: `runPipeline` states
which half of the pipeline the write lock is actually load-bearing for and names
its replacement (TKT-34XS2R); `flattenToLine` explains why neutralisation
happens at the splice site rather than inside `markdown.AppendToSection`;
`isWebhookConflict` records that only `ValidationErrorUnique` may be retried,
because retrying an ordinary validation failure would loop forever;
`webhookMaxInFlight` says it is sized for the mutex rather than for CPU.

## Project Documentation

- [x] `docs/webhooks.md` written (267 lines)
- [x] Limitations documented, not just capabilities

Covers the config reference, the three workflows, body encodings, the header
allowlist, the reserved `system:webhook:<id>` principal, and load shedding.

Two sections exist specifically to avoid overclaiming:

- **The concurrency table** states the guarantee *per tier*, including the row
that says an `append_section` can be lost across several rela processes on one
database — with the interim workaround (prefer `set:`, or run a single writer)
and a pointer to TKT-34XS2R.
- **The producer support matrix** names Icinga as working today and
Alertmanager / Grafana as not yet, because their payloads nest content in an
`alerts[]` array that interpolation cannot reach (TKT-ZEACWJ). It also warns
that an unresolved reference is an empty string rather than an error, so a
mistaken template creates entities with empty fields instead of failing.

Both were added because a reader who only saw the capabilities would configure
something that silently does the wrong thing.

## External Documentation

- [x] ~~README updated~~ (N/A: no top-level surface change — this adds a
config-declared endpoint, not a new binary, flag or command.)
- [x] ~~CHANGELOG~~ (N/A: the repo does not keep one; the ticket and commit
messages carry the history.)

## Cross-references

- [x] `CLAUDE.md` unchanged — deliberately. The webhook pipeline follows existing
rules (writes through `entitymanager`, `PatchEntity` over read-modify-write,
reads through the visibility wrapper) rather than introducing a new one, so
adding a section would restate what is already there. The one genuinely new
architectural rule to come — store-level CAS — belongs with TKT-34XS2R, which
implements it.
