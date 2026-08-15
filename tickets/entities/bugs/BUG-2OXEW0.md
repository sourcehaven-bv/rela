---
id: BUG-2OXEW0
type: bug
title: 'Unit tests issue real HTTP requests (unstubbed fetching children), flaking the Frontend CI job on unrelated PRs'
description: 'Component tests mount children that fetch in onMounted without stubbing them, so the suite issues real HTTP requests that fail ECONNREFUSED and log via console.error. When one lands after its test file finishes it races vitest worker teardown, reported as an EnvironmentTeardownError that exits 1 with every test passing, naming a file the PR author never touched. DynamicForm.test.ts (unstubbed SidePanel, 42 requests) is the file that surfaced it; EntityList.test.ts (unstubbed ExportMenu, 15 requests) was found by code review after the first fix. Fixed by failing closed on any unmocked request in test setup - per-file stub lists only ever cover the file that happened to lose the race.'
priority: medium
effort: s
why1: 'The Frontend CI job exits 1 with every test passing: an EnvironmentTeardownError from DynamicForm.test.ts.'
why2: 'A console.error settles while the vitest worker is closing its onUserConsoleLog RPC.'
why3: 'The log is SidePanel''s ''Failed to load side panel'' - the test never stubs SidePanel, so it issues a real HTTP request that fails ECONNREFUSED against happy-dom''s default origin (42 stray requests from one file).'
why4: 'SidePanel renders only when isEdit && entityId, and DynamicForm.test.ts is the only edit-mode file - it was written by copying a create-mode stub list. The same shape existed independently in EntityList.test.ts via ExportMenu, so this is a pattern rather than one file''s oversight.'
why5: 'Nothing makes a real network request fail loudly in unit tests. A component test can issue live HTTP and still pass - the request''s failure surfaces only as an async log, and only sometimes, in an unrelated place. The stub list is a per-file allowlist maintained by hand, so a newly-rendered fetching child is silently un-stubbed.'
prevention: 'Fail the test run on any real HTTP request rather than relying on per-file stub lists: a global test-setup guard that throws when the axios adapter or fetch is called with an unmocked URL turns a silent async log into a deterministic failure at the call site. Note the first fix attempt (afterEach unmount) was insufficient and measured 2/10 failures - unmount does not cancel an in-flight request. When a fix targets a race, measure the rate before and after over 20 runs rather than concluding from a handful of green runs.'
status: done
---

## Symptom

The Frontend CI job fails with **every test passing**:

```text
Test Files  101 passed (101)
Tests       1601 passed (1601)
Errors      1 error

⎯⎯⎯ Unhandled Rejection ⎯⎯⎯
EnvironmentTeardownError: [vitest-worker]: Closing rpc while
"onUserConsoleLog" was pending
This error originated in "src/components/forms/DynamicForm.test.ts"
```

Vitest counts an unhandled error as a failure, so the process exits 1.

Observed on PR #1310 — a **docs-only** PR whose entire diff is one markdown file
under `tickets/`, touching no frontend code. It passed on re-run of the same
commit with no changes, which is what confirms it as non-deterministic.

## Cause

> **The original diagnosis below was incomplete.** Leaked mounts are real, but
> unmounting alone does **not** fix the flake — measured at 2 failures in 10
> full-suite runs *after* adding `afterEach` teardown. The actual cause is an
> unstubbed child component, recorded first.

**Root cause: `SidePanel` fetches unstubbed.** `DynamicForm` renders
`<SidePanel>` only when `isEdit && entityId` (`DynamicForm.vue:1707`) — exactly
this test file's configuration — and `SidePanel` fetches
`/_sidepanel/<form>/<entity>` on mount. The test stubs `MarkdownEditor`,
`RelationPicker`, `RelationCards` and `AutoSaveIndicator`, but **not**
`SidePanel`, so the request is real: it hits happy-dom's default origin
(`localhost:3000`), fails `ECONNREFUSED`, and is logged by
`console.error('Failed to load side panel:', err)` (`SidePanel.vue:41`).

Measured: **42 stray `ECONNREFUSED` requests** from this one test file before
the fix, 0 after.

`await wrapper.unmount()` does not help, because unmounting does not cancel an
in-flight HTTP request — the rejection still settles and still logs. When it
lands after the file completes, vitest's `onUserConsoleLog` RPC is closing, and
whether it wins that race is timing-dependent. Hence intermittent, and hence
surfacing on PRs that changed nothing.

**Why only this file was ever named.** The sibling form tests
(`DynamicForm.cancel.test.ts`, `DynamicForm.embedded.test.ts`) mount in *create*
mode, where `isEdit` is false, so `SidePanel` never renders and never fetches.
They also already stub `@/api`. This file is the only one that mounts edit mode.

### Secondary: leaked mounts (the original diagnosis)

`DynamicForm.test.ts` mounts 15 times and never unmounts; `.cancel` and
`.embedded` leak their single mount too. Each leaked component keeps running
async work, and `DynamicForm.vue` logs from several async error paths —
`loadEntity` (`console.error`, ~:439 and ~:1059), template loading
(`console.warn`, ~:571), the create-mode dry-run (~:634), and auto-link
(~:1016).

Worth fixing on its own — a leaked component is a latent version of exactly this
bug for any future unstubbed call — but it is hygiene, not the cause.

## Why it matters beyond the noise

It red-flags unrelated PRs, and the failure text says nothing about the real
cause — it names a test file the author never touched. That is precisely the
shape of failure that trains people to re-run without reading, which erodes the
signal from every other CI failure.

## Fix

> **Revised after code review.** The first three items below fix the file named
> in the traceback. They are **not sufficient**: with them applied the full
> suite still issued **21 stray requests**, 15 of them from
> `EntityList.test.ts` via an unstubbed `ExportMenu` — the same `onMounted`
> fetch + `console.error` shape, and `api/transforms.ts` nulls its cache on
> rejection so every remount retries. Item 0 is what actually closes the class.

**0. Fail closed on any real HTTP request (`src/test/setup.ts`).** Set
`axios.defaults.adapter` to throw **synchronously at request time** on any
unmocked call:

```ts
axios.defaults.adapter = (cfg) => {
  throw new Error(`unmocked HTTP request in a unit test: ...`)
}
```

Synchronous is the point — the error lands at the call site, in the test that
caused it, instead of arriving as a nondeterministic teardown crash naming an
unrelated file. Note axios uses the **Node http adapter** under happy-dom, not
XHR, so patching `fetch`/`XMLHttpRequest` catches nothing.

Result: **21 stray requests → 0** across the suite, all 1662 tests still
passing — no test depended on making a real request. It also covers
`EntityList.test.ts` and `EntityList.cells.test.ts` without touching them
(neither has a `stubs` block), which is exactly the advantage over another
per-file stub.

A stub list is a denylist maintained by whoever last read a stack trace, so it
can only ever cover the file that happened to lose the race. That is why the
first attempt at this bug shipped with a live leak still in the tree.

**1. Stub `SidePanel`.** One line in `mountEdit`'s stub list:

```ts
stubs: {
  RouterLink: true,
  MarkdownEditor: true,
  RelationPicker: true,
  RelationCards: true,
  AutoSaveIndicator: true,
  SidePanel: true,   // ← renders only in edit mode; fetches on mount
},
```

Takes the file from 42 stray `ECONNREFUSED` requests to 0.

**2. Stub `getTemplates` via `@/api`**, matching what the sibling form tests
already do. Not load-bearing for *this* file (`loadTemplates` runs only in the
create branch, `DynamicForm.vue:1322`), but it removes a live network call the
moment anyone adds a create-mode case here.

**3. Unmount in an `afterEach`** — hygiene, applied to all three leaking form
test files:

```ts
const mounted: VueWrapper[] = []
afterEach(() => {
  mounted.forEach((w) => w.unmount())
  mounted.length = 0
})
```

Each file funnels every mount through one helper, so registering there covers
all call sites.

> The original ticket already flagged the possibility: *"an unmount stops the
> component but does not necessarily settle a promise already in flight."* That
> is exactly what happened — the unmount fix alone still flaked 2/10.

**Verification.** Two measurements, because the failure is a race and a handful
of green runs proves nothing:

| State | Full-suite failures |
|---|---|
| Baseline (no fix) | 1 in 6 |
| `afterEach` unmount only | **2 in 10** — insufficient |
| Unmount + `SidePanel` stub + `@/api` mock | 0 in 20 — but see below |
| + global request guard | **0 in 20**, and 0 stray requests suite-wide |

Plus the direct, deterministic measurement the race sits on top of — the number
to re-check if this regresses, since it does not depend on winning or losing a
timing race:

| Scope | Stray requests |
|---|---|
| `DynamicForm.test.ts`, before | 42 |
| `DynamicForm.test.ts`, after the stub | 0 |
| **Full suite**, with the stub but no guard | **21** (15 from `EntityList.test.ts`) |
| **Full suite**, with the guard | **0** |

The third row is the one that matters: it is what a code review caught and the
20-run measurement did not, because zero failures in 20 runs is consistent with
a leak that simply has not lost the race yet.

An early 5-run sample showed zero failures against the unmount-only fix and
nearly closed this bug on an incomplete diagnosis. A 20-run sample then showed
zero failures against a fix that still left 21 live requests in the suite.
**Green runs are weak evidence for a race; count the requests instead.**

## Provenance

The test file arrived with #1277 (BUG-MLT9DE). Not introduced by BUG-FB0LN8,
though that ticket's PR is where the flake was first observed.
