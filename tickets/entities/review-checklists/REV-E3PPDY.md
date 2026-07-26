---
id: REV-E3PPDY
type: review-checklist
title: 'Review: Move Lua action + webhook dispatch onto writeHandler'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test -race ./...` — full suite)
- [x] Lint clean (`golangci-lint run ./internal/dataentry/...`) — 0 issues
- [x] `gofmt` clean
- [x] `just plimsoll` passes — `App` directive ratcheted 115 → 114
- [x] `just arch-lint` passes
- [x] Builds across default / `postgres` / `memorybackend` tags

## Manual Review

- [x] `/code-review` run (cranky-code-reviewer, word-diff vs develop originals) — **zero findings**
- [x] Substitution fidelity exact — bodies byte-identical modulo receiver/field renames, each resolving to the same underlying value; lock scope, timeouts, params, correlation-id sites unchanged
- [x] Liveness verified — `luaDeps`/`fullScriptDetail` are method values reading swappable App fields live (post-`rebindApp` store/affordances, post-construction `SetSecurityConfig`); `engine` closure reads `app.scriptEngine` live (never reassigned)
- [x] Webhook principal stamping (`webhook:<event>`, `ToolWebhookReceiver`) outside the diff hunks — untouched
- [x] `writeMu` single instance; `actions.go`/`webhook.go` confirmed as the last two direct App takers — non-test `writeMu.Lock` sites now exist only on handler structs; no double-lock (both entry points top-level)
- [x] Wiring parity `NewApp` vs `rebindApp`; no stale doc claims actions/webhook live on App

**Review Responses:** none — zero findings. One non-blocking pre-existing
observation recorded on the ticket: store-wrapping tests mutate `app.store` in
place without rebuilding extracted handlers (inert for this path; same
pre-existing pattern as attachmentHandler).
