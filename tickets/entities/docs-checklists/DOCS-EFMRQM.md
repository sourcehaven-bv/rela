---
id: DOCS-EFMRQM
type: docs-checklist
title: 'Docs: Request-scoped Lua actions: full request in, arbitrary response out'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

`lua.Request` and `lua.WithRequest` carry the contract, and the godoc leads
with the decision a reader would otherwise reverse: `Request` is a VALUE and
deliberately not an `*http.Request`, so the lua package cannot be handed a live
request it could read a cookie or a bearer token off. Each field says when it
is present, because "nil" has three different meanings here (not requested,
empty, did not parse) and a script distinguishes them via `Raw`.

The decisions recorded where the mistake would be made:

- **`registerRequestBinding`** documents that the table is frozen for a reason
— it is the record of what actually arrived, so a script that rewrote it would
make a later read disagree with the wire — and that `freezeTable` does NOT
recurse, which is why each per-key list and the `_all` map are frozen
individually (RR-J1L1FS).
- **`DefaultActionMaxBodyBytes`** explains why the action endpoint gains a cap
it never had, and why it shares the webhook NUMBER but not the constant: the
two stay independently tunable while the shared ceiling stops either being
raised without limit.
- **`readActionPayload`** records the read-at-limit+1 trick and why a malformed
body is not an error for a request-scoped action but remains a 400 on the
legacy path.
- **`validActionContentTypes`** explains the allowlist and why the CSP and
`nosniff` are what make a script-controlled body survivable.

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new architectural rule.
An opt-in config block on an existing endpoint is the established shape.)
- [x] docs/ updated for changed behaviour
- [x] ~~Architecture docs updated~~ (N/A: no package boundary or dependency
direction change; `arch-lint` clean.)

`docs/` here is GENERATED from `docs-project/` entities by
`scripts/generate-docs.sh`, so the prose lives in
`docs-project/entities/guides/GUIDE-data-entry.md` and
`GUIDE-lua-scripting.md`, not in `docs/*.md`. This was corrected during review:
the branch had originally edited the generated files directly, which
`just docs-check` catches and which the next `just docs` run would have
silently reverted.

- **GUIDE-data-entry**: a "Request-scoped actions" section covering the
`request:` block, the `rela.request` table field-by-field with presence
conditions, the rich return shape and its defaults, the content-type
allowlist and response hardening, the error-mapping rule, and an explicit
"what this does not change" list (ACL gate, `capabilities:`, producer auth,
idempotency).
- **GUIDE-lua-scripting**: `rela.request` added to the globals table, stating
that it is absent outside a request-scoped action so `if rela.request then` is
the test.
- **`docs/webhooks.md`** (hand-maintained, not generated) points at the escape
hatch for payloads the declarative vocabulary cannot express.

## External Documentation

- [x] ~~README updated~~ (N/A: the feature is documented in the data-entry
guide; the README does not enumerate action config keys.)
- [x] ~~CLI reference updated~~ (N/A: no new or changed command or flag.)
- [x] API docs updated (the action endpoint's request and response contract, in
the data-entry guide)

## Rationale for N/A

`examples/icinga-alert.lua` is part of the documentation, not decoration: it is
the motivating use case from the ticket, and `actions_example_test.go` runs it
through the handler so the documented example cannot rot into something that no
longer works.

Idempotency is documented as explicitly NOT provided, with the quiet failure
mode named (a notification appended to an incident body twice) and the remedy
stated (match on a stable field before appending). Saying nothing would have
been the worse choice, since the failure is silent and a producer retry is
ordinary.
