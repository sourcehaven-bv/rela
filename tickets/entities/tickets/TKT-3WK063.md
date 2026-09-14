---
id: TKT-3WK063
type: ticket
title: Decide whether a missing id_type should stay a permanent hard load failure
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

`short_id_default.go:47-51` returns true for any entity lacking `id_type`,
indefinitely. The `auto`/`string` renames beside it are genuine legacy syntax
that a file either has or does not; the missing-key arm is a different thing: it
fires on every hand-written schema that omits an optional key whose default is
well defined.

Because `metamodel.FSLoader.Load` turns any detection into a `*migration.Error`,
that makes omitting an optional key a hard refusal to load. That is what made
BUG-0OTGGV fatal rather than cosmetic, what left three prototype schemas
unopenable, and what turned the documented idiom in `docs/metamodel.md` into a
trap.

Worth deciding deliberately, since there are defensible answers either way:

- If the intent is "we want every schema explicit", that is a lint, and it
should not be a hard load failure.
- If the intent is "the migration is transitional", the missing-key arm should
eventually retire while the value renames stay.

Needs a decision entity either way. Not urgent: BUG-0OTGGV made every in-tree
schema explicit, so nothing is currently failing.
