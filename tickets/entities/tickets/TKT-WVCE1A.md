---
id: TKT-WVCE1A
type: ticket
title: Ignore TypeScript major bumps until TS 7.1 lands a compiler API
kind: chore
priority: medium
effort: xs
status: review
---

## Problem

Dependabot keeps reopening TypeScript 7.0 major bumps that cannot pass CI:
[#1220](https://github.com/sourcehaven-bv/rela/pull/1220) (`/e2e`) and
[#1227](https://github.com/sourcehaven-bv/rela/pull/1227) (`/frontend`). Both
fail `npm ci` with `ERESOLVE`.

The cause is upstream and not a version-range oversight: **TypeScript 7.0 ships
no JS compiler API at all.** `typescript-eslint` therefore caps
`peerDependencies.typescript` at `>=4.8.4 <6.1.0` — still true in 8.67.0 and in
the canary line. Forcing the install past it does not help; `typescript-eslint`
has a hard runtime guard that refuses to load:

```
Error: typescript-eslint does not support TS 7.0.
```

Upstream ([typescript-eslint#10940](https://github.com/typescript-eslint/typescript-eslint/issues/10940))
is explicit that nothing can be done until TypeScript ships the new API, which
Microsoft expects in **TS 7.1**. The thread is locked.

Both `/frontend` and `/e2e` depend on `typescript-eslint`; in `/e2e` it also
enforces the Page Object Pattern lint rules.

Note `/frontend` already pins `typescript: ~6.0.3` (patch-only) and Dependabot
proposed the major anyway — a package.json range does not constrain it, so an
`ignore` rule is what actually stops the churn.

## Fix

Add a `version-update:semver-major` ignore for `typescript` to both npm blocks
in `.github/dependabot.yml`. Scoped to majors only, so TS 6.x patch/minor
updates keep flowing. The `build-tooling` group is already `minor`/`patch`-only,
so no group PR can smuggle the major back in.

## Revisit

When TS 7.1 ships the new compiler API, delete the two `ignore` entries — that
is the whole change. The e2e sources were verified to typecheck clean under TS
7.0 (`tsc --noEmit`, 0 errors across 52 files), so the eventual bump should be
mechanical, modulo switching `e2e/tsconfig.json` off `moduleResolution: node`
(removed in TS 7, `error TS5108`) to `node16`.
