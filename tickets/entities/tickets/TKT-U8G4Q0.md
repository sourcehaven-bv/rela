---
id: TKT-U8G4Q0
type: ticket
title: 'Fix develop CI: policy-less scriptReader test vs visibility.Unrestricted (semantic conflict PR #1204 × #1208)'
kind: test
priority: high
effort: xs
status: done
---

Develop's Test job has been red since #1216/#1217/#1218 merged on top of the
#1204 × #1208 pair. Root cause is a **semantic conflict**, not a bad change:

- **#1204** (`fix(acl): fail closed when the data-entry script read gate
cannot be built`) added `TestScriptReadSeam_PolicylessProjectStaysUnrestricted`
with the assertion `any(reader) != any(app.store)` — "policy-less scriptReader
should be the raw store".
- **#1208** (`refactor(visibility): name the ungated read path`, TKT-1WV50C)
deliberately changed `App.scriptReader`'s NopACL branch to return
`visibility.Unrestricted(a.store)` — a named, greppable ungated read handle. Its
branch predates #1204's merge, so neither PR's CI saw the combination.

## Fix

Update the test to pin the CURRENT intended contract: the policy-less path must
return `*visibility.UnrestrictedReader` (and still never `DenyReader`). Behavior
is unchanged — the wrapper is pass-through by design (pinned by
`internal/visibility/unrestricted_test.go`).

## Verification

- `go test -race ./internal/dataentry/` — full package green
- `golangci-lint run ./internal/dataentry/` — 0 issues
