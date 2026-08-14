---
id: prototype-configs-parse-test
type: automated-measure
title: Prototype data-entry.yaml configs parse without a browser or built SPA
description: Parses every prototypes/**/data-entry.yaml through dataentryconfig and fails on an unmarshal error. Needs no browser and no built SPA, unlike the docscapture tests that consume the same configs — so a schema migration that misses a prototype config fails in the PR that introduced it rather than lying dormant behind a skip.
kind: test
location: internal/dataentryconfig/prototype_configs_test.go (proposed — does not exist yet)
status: proposed
---

A browserless, build-free test that parses every `prototypes/**/data-entry.yaml`
through `dataentryconfig` and fails on an unmarshal error.

## Why this measure

The `internal/docscapture` tests that consume the prototype project are gated
behind `requireBrowser`, which skips unless **both** a Chrome binary and a built
SPA are present. CI has neither, so a schema migration that misses the prototype
config (BUG-JWTKVG: `command:` string → `[]string` in #1284) lands green and
stays broken indefinitely — surfacing only for a developer who has run `just
build-frontend` locally.

A plain parse test has none of those preconditions, runs in milliseconds, and
would have failed in the same PR that introduced the schema change.

## Scope

Deliberately narrower than "make docscapture run in CI" — that is a heavier,
separate decision (browser in CI, frontend build ordering). This measure only
guarantees the in-tree example configs stay loadable against the current schema,
which is the specific rot that occurred.
