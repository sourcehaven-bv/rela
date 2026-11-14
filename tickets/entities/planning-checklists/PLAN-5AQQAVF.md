---
id: PLAN-5AQQAVF
type: planning-checklist
title: 'Planning: rela test: stub and record outgoing HTTP calls'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: `t.http.stub`, recorded calls, `t.secrets`, and running flow job steps
under `rela test` (`actor:start_flow`, `t.flows.work`, `t.flows.run`), so script
jobs and `kind: webhook` jobs go through the stubs.

Out of scope: AI stubs (TKT-2NO1LXC); the guide, CLI reference and sample suite
(TKT-E2IMTMG, which gets the guide notes in TKT-HB313W5's body); scheduler ticks
(TKT-NA2FG3A).

**Acceptance Criteria:**
1. A stub matches on method, exact URL (query order ignored), prefix or whole-URL RE2 pattern, headers, exact body, JSON subset and form fields.
2. A stub answers with status, headers and body / json / fixture file, with a `timeout` or `network` error, or with a sequence.
3. An unmatched call fails the case with the stubs listed, also when the script swallows the error; a spent sequence says so.
4. `stub:calls()`, `stub:count()` and `t.http.calls()` return the calls in order with method, url, headers, body, json and stub.
5. `t.secrets` feeds scripts and webhook signatures; `.rela/secrets.yaml` is never read.
6. A stubbed 503 then 201 drives a flow script job's retry at the backoff on the case's clock; a 4xx fails it as permanent; a webhook retries a network error.
7. A fixture file outside the project or behind a symlink is refused.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the shape follows the existing seams; no open design question)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** httptest servers and gock/httpmock were considered. A
real listener is not wanted (a case must not open sockets) and the seam already
takes an `*http.Client`, so a custom `http.RoundTripper` is enough and adds no
dependency. The matching and sequence model follows nock and WireMock (first
match wins, scenario-style sequences). Reused: `appbuild.WithHTTPClient`,
`appbuild.WithHostConfig`, `flowengine.AttemptHandler`, `noSymlinks` and
`rootfs` from `t.fixtures`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `internal/apptest/httpstub.go`: a per-case `stubTransport` replaces the refusing transport. It records every call, answers from the first matching stub that is not spent, and records the first unmatched call as the case's failure, which wins over later errors.
- `internal/apptest/host.go`: the case's host config serves mail.yaml as before and the `t.secrets` values.
- `appbuild.WithFlowJobQueue`: the flow engine puts attempts on a caller's queue. The test world has no job queue or flow worker; `t.flows.work()` runs queued attempts through `flowengine.AttemptHandler` and `Engine.Due` on the case's goroutine.
- `flowengine.HandleAttempt` bounds the attempt's context by the time left on the engine's clock instead of the absolute deadline, because the engine's clock is pinned in a test and the context runs on the wall clock.

Alternatives rejected: running the in-memory queue in the background (cases
would race the job; the clock would not drive retries); turning on
`WithBackgroundAutomationJobs` (changes automation semantics in every case); a
Lua-side mock of `http.*` (would miss webhook jobs, which post from Go).

**Files to modify:**
`internal/apptest/{httpstub,httplua,host,flows,world,file,module,actor,network}.go`,
`internal/appbuild/{seams,flowruntime}.go`, `internal/flowengine/job.go`,
`.go-arch-lint.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** stub specs come from operator-authored test
files; unknown keys, bad statuses, bad error kinds and invalid patterns are
argument errors. Fixture files go through the `t.fixtures` path rules (local
path, no symlink component, `rootfs` read).

**Security-Sensitive Operations:** no network access: the transport never dials.
Secrets: the case's host never reads `.rela/secrets.yaml`, so recorded calls and
failure messages only hold values the test declared. A script still sees only
the secrets its capabilities name.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** `internal/apptest/httpstub_test.go` runs `.test.lua` files:
AC1-2, 4 in `TestHTTPStub_Answers`; AC3, 7 and spec errors in
`TestHTTPStub_Failures` and `TestHTTPStub_FixtureSymlinkRefused`; AC5 in "no
real secrets" and `TestHTTPStub_WebhookSignedWithFakeSecret`; AC6 in
`TestHTTPStub_FlowJobRetry` (29 s does not retry, 30 s does).

**Edge Cases:** two stubs with the same generated name (numbered); a stub with
no answer (200, empty body); a spent sequence; a request without a body;
`t.flows.run` for an unknown id (nil).

**Negative Tests:** unmatched call, swallowed error, header mismatch, two URL
keys, unknown key, bad error kind, duplicate explicit name, fixture outside the
project, fixture symlink.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** `t.flows.work()` could loop on a flow that keeps queueing at one
instant; bounded at 100 rounds with an error. The flowengine deadline change is
behaviour-neutral on the wall clock (engine clock = wall clock outside tests);
the flowengine and appbuild flow tests pin it.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** no user docs for `rela test` exist yet; the guide is
TKT-E2IMTMG. This ticket writes the godoc reference on `fileRun.module` and the
guide notes in its body, and TKT-E2IMTMG's description points at them.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** the design was reviewed together with the code in
the review phase (see the review-responses on TKT-HB313W5), because the
implementation was small enough to review as one change.
