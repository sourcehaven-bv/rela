---
id: TKT-QGHNVA
type: ticket
title: Stop passing user-controlled IDs on the command line (use temp file)
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

`internal/dataentry/document.go:renderCommand` splices the request-derived
entity ID into the operator's `command:` string, which `executeCommand`
(`document.go:410`) runs via `exec.Command("sh", "-c", …)`.

That makes the entity ID the one piece of user-controlled data that reaches a
shell. Everything protecting that today is *input filtering*:
`isSafePathSegment` (a third ID validator, distinct from the two in TKT-IZGF7T),
plus the `#nosec G702` rationale at `document.go:401-409`.

Filtering is the weaker control. A leading `-` passes every current check, so an
entity id like `-rf` or `-oevil` lands in the command string positioned as an
**option flag** rather than an operand — argument injection into whatever tool
the operator invokes. This was raised in IB-review on PR #1248.

### The design fix

Don't pass user-controlled data on the command line at all. If the ID (or the
entity content) is delivered to the external command via a temp file — the
pattern `internal/cmdexec` already uses with its `{in}`/`{out}` placeholders —
then no amount of validator weakness can produce argument injection. The class
of bug disappears rather than being filtered against.

This is worth doing *because* it is defence in depth: TKT-IZGF7T tightens the
validators, but the guarantee then rests on three separate validators staying
correct forever. Removing the untrusted input from the command line removes the
dependency.

Note the ID is in the entity file anyway, so a command that needs it can read it
from the file it is already being handed.

### Scope

IN scope:
- Remove request-derived data from the `sh -c` string in the document
render path.
- Reuse the existing `internal/cmdexec` temp-file pattern
(`{in}`/`{out}`, argv array, no shell) rather than inventing a second mechanism.
- Update the `#nosec G702` rationale to match whatever remains true.

Explicitly to decide during planning:
- `{id}` / `{id_lower}` are a **documented operator-facing contract**
(`docs/data-entry.md:2255-2278`: `command: "my-renderer {id}"`). Removing or
changing them breaks existing operator configs. Options: keep `{id}` but pass
via file/env, deprecate with a migration window, or provide `{in}` alongside and
phase `{id}` out. This is a backwards-compatibility decision, not purely a
security one.

NOT in scope:
- Validator unification (TKT-IZGF7T).

### Relationship to TKT-IZGF7T

TKT-IZGF7T is the *filtering* fix and should land first — it is small and closes
the immediate finding. This ticket is the *structural* fix that makes the
filtering non-load-bearing. Both are wanted; neither replaces the other.

### Acceptance criteria

1. No request-derived value is interpolated into a string passed to
`sh -c` on the document render path.
2. An entity whose ID begins with `-` cannot influence the argument
structure of the rendered command (regression test).
3. The backwards-compatibility decision for `{id}`/`{id_lower}` is
documented and implemented, with `docs/data-entry.md` updated.
4. The `#nosec G702` comment states only what is actually guaranteed.
