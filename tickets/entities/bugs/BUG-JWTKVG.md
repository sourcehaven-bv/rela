---
id: BUG-JWTKVG
type: bug
title: 'Prototype data-entry.yaml still uses the old string command: form, breaking docscapture'
description: '#1284 changed documents.<name>.command from a shell string to []string argv, but prototypes/data-entry/project/data-entry.yaml was not migrated. internal/docscapture uses that project as its capture corpus, so all its browser tests fail at app-build time. CI stays green because those tests skip unless a built SPA AND a browser are both present.'
priority: medium
status: backlog
---

## Symptom

Every browser-dependent test in `internal/docscapture` fails:

```
capture_test.go:65: Capture: build data-entry app: parsing data-entry.yaml:
  yaml: unmarshal errors:
  line 44: cannot unmarshal !!str `export ...` into []string
```

Affects `TestCapture_Form`, `TestCapture_AnnotationAndFailLoud`,
`TestCapture_SeedGrowsAcrossIslands`,
`TestCapture_UnrenderableEntity_FailsLoud`.

## Cause

`#1284` (37c66a69, "document renderers run without a shell — remove `{id}` from
the command line", TKT-QGHNVA) changed `documents.<name>.command` from a shell
string to a `[]string` argv. `prototypes/data-entry/project/data-entry.yaml` was
not migrated: `ticket_summary.command` (line ~44) is still a multi-line shell
block (`export PATH=...`, `rela show {id} -o json | jq ...`), which no longer
unmarshals.

`internal/docscapture` uses that prototype project as its capture corpus
(`protoDir` → `../../prototypes/data-entry/project`), so every capture fails at
app-build time.

## Why CI is green

`requireBrowser` skips unless **both** a Chrome/Chromium binary **and** a built
SPA (`dataentry.CheckEmbeddedSPA`) are present. CI's Go test job has no built
frontend, so these tests skip and the breakage is invisible.

It surfaces locally for anyone who has run `just build-frontend` and has Chrome
— which is exactly the developer doing UI work.

## Reproduction (confirmed on clean develop, no feature branch involved)

```sh
git worktree add /tmp/devcheck origin/develop
cd frontend && npm ci && npm run build      # or copy an existing static/v2
cd /tmp/devcheck && go test ./internal/docscapture/ -run TestCapture_Form
```

Verified: SKIPs without a built SPA, FAILs with one, on `origin/develop`
(35374c58) with no other changes. Found while rebasing TKT-53KICM; that branch
does not touch the file (`git diff --name-only origin/develop...HEAD` has zero
`prototypes/` entries).

## Fix

Migrate `ticket_summary.command` in the prototype config to the argv form the
other documents now use, per the TKT-QGHNVA migration. Note the command relies
on a shell (`export`, `|`, `$(...)`, `{id}` interpolation) — precisely what that
ticket removed — so it needs rewriting as a script invocation rather than a
mechanical YAML reshape.

## Prevention candidate

The skip-unless-built gate means a whole package can rot unnoticed. Worth
considering either a CI job that builds the frontend before running
`docscapture`, or a cheap config-parse test over `prototypes/**/data-entry.yaml`
that needs no browser and would have caught this at the moment #1284 landed.
