---
id: IMPL-S2MCF1
type: implementation-checklist
title: 'Implementation: Operator customisation hooks: serve + inject custom.css/custom.js, @layer cascade fix, isCustomElement'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written (Go: 9 new test funcs in custom_test.go / custom_shell_test.go /
      custom_layer_test.go; frontend: 41 cases in relaCssLayer.test.ts, 3 in RelaSlot.test.ts)
- [x] Integration tests written (5 Playwright e2e in customisation.spec.ts driving a real browser
      against the real server binary)
- [x] Feature implemented
- [x] Edge cases from planning handled (empty file, directory, oversize, symlink escape, missing
      insertion markers, disabled-injection, client-side route, real-asset passthrough)

## Manual Verification

Verified against a live `rela-server` on a real project, not just tests:

- [x] Traversal: `/_custom/secret.txt`, `/_custom/../secret.txt`, `%2e%2e%2f`, `..%2F`, and
      `custom.css/../secret.txt` — **no spelling leaked project-root content**.
- [x] Headers on a served asset: `Content-Type: text/css; charset=utf-8`,
      `X-Content-Type-Options: nosniff`, `Cache-Control: no-cache`.
- [x] Injection: `<link rel="stylesheet" href="/_custom/custom.css">` present with the file,
      absent without it; `custom.js` added while the server was RUNNING was picked up with no
      restart (confirms the per-request stat design).
- [x] **AC3 in a real browser**: operator `.sidebar` (0,1,0, unlayered) beat rela's actual
      `.sidebar[data-v-f40ef6f2]` (0,2,0, layered) — `operatorWins: true` — both before and after
      a route chunk was appended to `<head>`.
- [x] Build output: 19/19 stylesheets layered; all 4 `:root` token blocks unlayered, 0 inside.

## Quality

- [x] Follows project patterns (`os.OpenRoot` idiom from `openAppEntry`; uniform 404;
      `apps_test.go` table-test conventions; e2e page-object rule)
- [x] No silent failures — every `openCustomAsset` error path is an explicit 404, and the
      `@layer` invariant is asserted over real build output rather than assumed
- [x] `just lint` 0 issues, `just arch-lint` clean, `npm run lint` 0 errors, typecheck clean
