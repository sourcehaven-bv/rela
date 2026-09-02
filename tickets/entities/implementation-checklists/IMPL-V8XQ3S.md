---
id: IMPL-V8XQ3S
type: implementation-checklist
title: 'Implementation: idp-sync example: validate webhook claims before interpolating them'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written~~ (N/A: an example Lua script; the repo has no test
harness for `examples/`. The pattern itself WAS verified — see evidence.)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented — an allowlist regex on both identifiers, before
either interpolation site.
- [x] Edge cases from planning handled — see the verification table.
- [x] Error handling in place — returns the script's existing structured error
shape (`message_type = "error"`), matching the two validation failures already
above it, rather than erroring in a new way.

## Test Quality

- [x] ~~Fixture builders~~ (N/A)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The pattern is the whole change, so it was executed against a real `lua`
interpreter rather than eyeballed — a Lua character class is easy to get subtly
wrong, and a too-narrow one silently rejects legitimate users:

| input | accepted | note |
|---|---|---|
| `user@example.com` | ✅ | email subjects |
| `a-b_c.d` | ✅ | slugs |
| `01H8XGJ` | ✅ | ULIDs / UUIDs |
| `ABC-123` | ✅ | |
| `../etc/passwd` | ❌ | path traversal |
| `a/b` | ❌ | path segment injection |
| `a?b`, `a#b` | ❌ | query / fragment injection |
| `a b` | ❌ | space |
| `a\nb` | ❌ | newline |
| `` (empty) | ❌ | already caught above, belt-and-braces |
| `auth0\|abc123` | ❌ | **see below** |

All twelve behaved as intended.

**The Auth0 case is called out in the comment rather than accommodated.**
`auth0|abc123` is a common real subject format and this pattern rejects it. That
is a deliberate default: `|` is harmless in a path, but widening the set to
"characters that happen to be safe today" is how an allowlist decays into a
blocklist. The comment names the case, says to widen the pattern if your IdP
issues such subjects, and says to do it character by character — never `.*`.

## Quality

- [x] Code follows project patterns — reuses the script's existing structured
error return; no new failure mechanism.
- [x] Checked for DRY opportunities — one `valid_id` helper covering both
identifiers, rather than two inline matches.
- [x] No security issues introduced — this **adds** a control. Allowlist, not
blocklist, per the project's own design-review guidance.
- [x] No silent failures — an invalid identifier returns a named error rather
than being silently dropped or sanitized into something else.
- [x] No debug code left behind.

**Scope note.** This is defence in depth, not the primary control: the webhook
JWT is already cryptographically verified (ES256, with a confused-deputy guard
via a separate audience), which is why the issue rates it Low. The comment says
so, so nobody reads the check as *the* protection and relaxes the JWT
verification.

The reason it is still worth doing: this is an **example**, and examples get
copied. Whoever adapts it inherits whatever it models.
