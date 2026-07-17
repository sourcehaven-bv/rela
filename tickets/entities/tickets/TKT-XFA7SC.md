---
id: TKT-XFA7SC
type: ticket
title: 'Arch-lint/CI rule: forbid the Create-then-Update-on-ErrConflict fallback idiom'
kind: chore
priority: low
status: backlog
---

## Description

Systemic (Why-5) prevention for the create-then-Update-on-ErrConflict upsert bug
class — BUG-ZWTDH9 (entitymanager) and BUG-5QDV6F (rename). Both were the same
copy-pasted idiom: `CreateX` → on `store.ErrConflict` fall through to `UpdateX`,
which silently clobbers a racing writer on the multi-writer postgres backend.
Each was found and fixed by hand; nothing stops a future copy.

**Proposal:** add a lint/CI guard (a grep-based CI check, or a
go-arch-lint/custom analyzer) that flags any `errors.Is(err, store.ErrConflict)`
branch leading to an `UpdateEntity`/`UpdateRelation` call, **except** the
sanctioned resolve-by-intent path in `entitymanager/apply.go` (which writes by a
pre-resolved create-XOR-update op and never falls through on conflict). A future
reintroduction should fail CI rather than depend on a reviewer remembering
BUG-ZWTDH9.

**Scope note:** decide the mechanism during planning — the grep gate is cheap
and matches existing CI guards (e.g. `rela-ticket-check`); a proper analyzer is
sturdier but heavier. Originated as the P5 item on BUG-5QDV6F.
