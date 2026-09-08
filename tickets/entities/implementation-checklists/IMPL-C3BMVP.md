---
id: IMPL-C3BMVP
type: implementation-checklist
title: 'Implementation: Reject cleartext http:// for plantuml_server_url except loopback'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) — verified through `rela validate` against a real project, see evidence
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (table of URL strings; no objects to build)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (the URL under test IS the input)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (assertion is a substring of the error message)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `cmd/rela` from this branch and ran `rela validate` against a scratch
project, rewriting `data-entry.yaml` per case. (`rela list` does NOT validate
data-entry.yaml — an early probe against it accepted everything including
`javascript:`, which the *pre-existing* check rejects. That control is what
exposed the probe as useless; `validate` is the real surface.)

| Input | Result |
| --- | --- |
| `https://plantuml.example.com` | ACCEPT (AC1) |
| `http://localhost` | ACCEPT (AC2) |
| `http://LOCALHOST:8080` | ACCEPT (AC2, case-insensitive) |
| `http://127.0.0.1:8080` | ACCEPT (AC2) |
| `http://127.1.2.3:9000` | ACCEPT (AC2, rest of 127/8) |
| `http://[::1]:8080` | ACCEPT (AC2, IPv6 brackets stripped) |
| `http://plantuml.example.com` | REJECT (AC3) |
| `http://93.184.216.34` | REJECT (AC3, public IP) |
| `http://192.168.1.10:8080` | REJECT (AC3, RFC1918 gets no exemption) |
| `http://localhost.evil.com` | REJECT (AC4) |
| `http://127.0.0.1.evil.com` | REJECT (AC4) |
| `http://localhost@evil.com` | REJECT (AC4) — error names host `evil.com`, so `Hostname()` unmasked the userinfo |
| `http://evil-localhost.com` | REJECT (AC4) |
| `javascript:alert(1)` | REJECT — pre-existing message unchanged (AC5) |
| `https://` | REJECT — pre-existing message unchanged (AC5) |

AC6: `docs/data-entry.md` regenerated from `docs-project/` via `just docs`.

Mutation testing (both directions, each confirmed applied to real code):
1. Disabled the new `case` → the 7 rejection rows reddened; loopback rows stayed green.
2. Replaced the exact match with `Contains`/`HasPrefix` → exactly the 3
lookalike-host rows reddened. (A first attempt at this mutation failed to
compile — `net` went unused — which proves nothing, so it was redone in a form
that compiles.)

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
