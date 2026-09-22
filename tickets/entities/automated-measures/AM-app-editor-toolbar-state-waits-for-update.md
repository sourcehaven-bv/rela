---
id: AM-app-editor-toolbar-state-waits-for-update
type: automated-measure
title: App-editor toolbar assertions wait for the ProseMirror update cycle
description: The two toolbar tests use a retrying assertion (expect.poll or a web-first assertion on the pressed state) rather than a bare expect immediately after the click, so they wait for ProseMirror update cycle instead of racing it. Verified by --repeat-each over the pair staying green.
kind: test
location: e2e/tests/apps.spec.ts
status: proposed
---

The two `apps.spec.ts` toolbar tests assert through a retrying matcher rather
than a bare `expect(...)` taken immediately after `clickEditorLine`.

Verification is a `--repeat-each=10 --workers=5` run over `apps.spec.ts:182` and
`apps.spec.ts:201` staying green — the command that currently reproduces the
flake 3 times in 20.
