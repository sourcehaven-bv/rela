---
id: DOCS-L86U7N
type: docs-checklist
title: 'Documentation: acl-query audit op'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — the call site says **why the record
is written before the answer** (an attempt to enumerate access for a
non-existent id is as interesting as a successful one; recording only successes
would let a prober stay out of the log).
- [x] Function/type docs if public API — `audit.OpACLQuery`'s godoc is the
load-bearing one. It carries the decision that will otherwise be re-litigated:
**the result is deliberately not recorded**, because the answer is a list of
principals and their access routes, so logging it would duplicate the disclosure
rather than record it. The same rule `OpACLBypassRead` already applies to
elevated reads, and the godoc cites it so the two stay consistent.

Without that sentence, "improving" the record by adding the result looks like an
obvious enhancement.

## Project Documentation

- [x] ~~README updated~~ (N/A)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; follows the existing audit-op
conventions)
- [x] ~~Help text accurate~~ (N/A: no flag or command change — `who-can`'s
surface is identical)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no CHANGELOG)
- [x] API docs updated — `docs-project/entities/guides/GUIDE-audit-log.md`,
regenerated into `docs/audit-log.md`.

A **new op** belongs in the guide, unlike the earlier attachment ticket which
only added an occurrence of an existing one.

While adding it, the field table's inline `op` list turned out to be **already
stale** — it named eight ops and omitted `acl-bypass`, `acl-bypass-read`,
`purge-version`, `data-migration` and `data-gc`. A one-line cell was the wrong
shape for a growing set, so it now points at a proper Operations table listing
every op with its meaning. That is a small scope extension beyond this ticket,
taken because adding a ninth op to a list that was already wrong would have made
the guide *more* misleading, not less.

The `acl-query` row states the omission explicitly, so an operator reading the
guide learns the answer is not in the log rather than assuming it is and
wondering why they cannot find it.
