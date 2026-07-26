---
id: IMPL-OZELQR
type: implementation-checklist
title: 'Implementation: fail-closed data-entry script read wiring'
status: done
---

## Development

- [x] `App.scriptReader` returns `visibility.DenyReader{}` on all three construction faults, logged at `slog.Error`.
- [x] `App.scriptTracer` returns `visibility.DenyTracer{}` on both faults.
- [x] NopACL early-return preserved unchanged in both.
- [x] Godoc rewritten to state the real justification (the unattended webhook consumer) and the deliberate divergence from appbuild's nil-redactor guard.

## Quality

- [x] `go build ./...` clean.
- [x] `just lint` — 0 issues (fixed one `unparam` on the test helper).
- [x] `just arch-lint` — no warnings.
- [x] `just plimsoll` — no new god-object violations.
- [x] Tests: dataentry, visibility, appbuild, lua, scheduler all pass.
- [x] Mutation-tested: reverting to fail-open (and separately breaking the NopACL early-return) makes each test fail with an actionable message.
- [x] ~~Frontend changes~~ (N/A: Go wiring only)

## Notes

`internal/docscapture` fails locally because `frontend/dist/` is not built in
this checkout. Verified byte-identical failure with the branch stashed (tree ==
develop), so it is a pre-existing local-environment gap, not a regression. CI
builds the frontend.
