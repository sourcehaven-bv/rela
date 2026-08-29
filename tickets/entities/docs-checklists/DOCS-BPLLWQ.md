---
id: DOCS-BPLLWQ
type: docs-checklist
title: 'Docs: Attachments on entity create'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] New/changed functions have doc comments
- [x] Non-obvious decisions explained with rationale

The comments carry the *reasons*, not restatements: why staged files must stay
out of `formData` (they would be POSTed as the property value), why the
statement order in `handleSubmit` is load-bearing, why `shallowRef` rather than
`ref` (test/production `File` divergence), why removal is by index rather than
identity, and why `attachmentErrorReason` duck-types on `status`.

## Project Documentation

- [x] `docs/data-entry.md` — new "File Properties on Forms" section explaining
create-vs-edit behaviour, the "Pending save" state, capacity, and the
partial-failure case. Authored in
`docs-project/entities/guides/GUIDE-data-entry.md` and regenerated; `just
docs-check` passes.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change — `file` and `max` already documented)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`docs/attachment-security.md`~~ (N/A: the security model is deliberately
unchanged — no new endpoint, blob namespace or ACL subject. That is the whole
argument for this approach, so documenting a change would be wrong.)
- [x] ~~`CLAUDE.md`~~ (N/A: no new cross-cutting pattern; the two-phase create
mirrors the existing `link_as=from` relation write)
- [x] ~~`README.md`~~ (N/A: no project-level change)

## Accuracy

- [x] Docs describe what the code actually does

Worth recording: the first draft of this section **promised a retry that did not
exist** — "the message names the files that did not attach, so you can retry
them there" — while the code cleared nothing and navigated away. Code review
(RR-4QO887) caught the divergence. The section now says the failed files must be
picked again, because the form holding them is gone by then. A doc that
overstates a recovery path is worse than one that omits it.

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: this repo has no changelog; the ticket
and commit message carry the rationale)
