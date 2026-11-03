---
id: PLAN-MUHW0I
type: planning-checklist
title: 'Planning: End SSE streams when the verified token expires'
started: "2026-10-20"
completed: "2026-10-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** the JWT gate (`requireVerifiedJWT`, internal/dataentry/jwtgate.go)
checks the assertion's `exp` once, when a request starts. The events stream
(`handleSSE` and `runSSELoop`, internal/dataentry/watcher.go) then lives until
the client goes away, a config reload retires the broker, or the server stops.
On `/api/v1/_events` the live-view hub replays GETs as the stream's owner
(`liveConn.owner`, live.go) for the whole lifetime. Channel and presence changes
reach the client the same way. So a stream keeps receiving data computed for a
principal whose token has expired, and whose IdP claims may have been revoked.

**Scope:**
- In: carry the verified assertion's expiry from the JWT gate to the request,
and end the events stream (`/api/events` and `/api/v1/_events`) when it passes.
Unregistering the live connection on return stops its replays.
- Out: the other auth modes (trusted header, env user, desktop, the default
`unknown` principal). They have no expiry; their streams behave as today.
- Out: the command-exec SSE (`handleCommandExec`). It streams the output of one
action the caller started with a valid token, and it ends when the script ends.
- Out: the MCP `watch` long-poll. It replays as the caller for at most 50 s
per call, and the next call goes through the gate again.
- Out: SPA changes. `useEvents.ts` already reconnects with backoff when the
stream ends, and on reconnect the proxy supplies a fresh assertion (rela sets no
cookies). Live-view subscriptions are re-PUT on reconnect (api/live.ts).
- Out: the deprecated `JWTPrincipalResolver`. It falls back to the next
resolver, and the gate replaces it.

**Acceptance Criteria:**
1. A stream opened through the JWT gate ends at the assertion's expiry.
Test: an SSE test with a stub verifier that returns an expiry a short time
ahead. The response body reaches EOF after the expiry, and the live connection
is unregistered.
2. A stream without an expiry (no JWT gate) does not end on its own. Test: the
same stream without the gate stays open past the same interval, and pings still
arrive.
3. An expiry already in the past when the stream starts ends it before
anything is registered or written: `handleSSE` checks it before subscribing to
the broker and before `live.register`. Test: stub expiry in the past; the
response has no frames and the hub has no connection (the real verifier rejects
this, so the case only guards the handler).
4. The verifier projects `exp` into `AssertionClaims.ExpiresAt`, and the
rela-server adapter copies it into `AssertedIdentity.ExpiresAt`. Test: a jwtauth
test that mints a token with a known `exp`; an adapter test in cmd/rela-server.
5. The expiry does not change identity: a live-view PUT with a refreshed token
(later `exp`, same claims) is not answered with `stream_principal_stale`. Test:
live PUT test where the stream and the PUT carry different expiries.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change inside RES-1UCUBA's design)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: a timer in an existing select loop; golang-jwt already parses `exp`)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: ending a stream at credential expiry is the common pattern, e.g. Kubernetes watch with bound tokens; nothing to reuse)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (RES-1UCUBA covers live views)

**Existing Solutions:**
- `jwt.MapClaims.GetExpirationTime` (golang-jwt v5) returns the parsed `exp`;
the parser already requires it (`jwt.WithExpirationRequired`,
internal/jwtauth/verifier.go).
- The SSE loop already selects over timers (`flush`, `ping`) in
`runSSELoop`; an expiry timer is one more case.
- Request-scoped values set by the gate and read by handlers follow the
`principal.With` / `readGateFromContext` pattern.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `jwtauth.AssertionClaims` gains `ExpiresAt time.Time`, set from
`claims.GetExpirationTime()` in `VerifyAssertion` (zero when absent; the parser
requires it, so absent only happens with a custom option set). Its doc defines
it as the moment the verifier stops accepting the token: `exp` plus any leeway
the parser is given. The verifier sets no leeway today, so it equals `exp`; a
leeway added later must be added here too, or streams opened inside it would
close at once and reconnect in a loop.
2. `dataentry.AssertedIdentity` gains `ExpiresAt time.Time`; the adapter in
cmd/rela-server copies it.
3. dataentry gets an unexported context key for the credential expiry, with
`withCredentialExpiry(ctx, t)` and `credentialExpiry(ctx) (time.Time, bool)`.
`requireVerifiedJWT` sets it next to `principal.With` when `ExpiresAt` is not
zero.
4. `handleSSE` reads `credentialExpiry(r.Context())` first. An expiry that is
not in the future returns before the broker subscription and `live.register`.
Otherwise `runSSELoop` arms a `time.Timer` for `time.Until(exp)` and returns on
it, like the `ctx.Done()` case. No final frame: the client sees the end of the
stream and reconnects, as for a reload.
5. A frame write can block for up to the write deadline (50 s), and the select
does not run meanwhile. `sseConn` gets the expiry and caps each write deadline
at it, so a blocked write fails at the expiry and the loop returns.
6. The expiry is NOT put on `principal.Principal`. `Equal` (used by the live
PUT owner check), `ShareKey`, the JSON form (jobs, audit) and `Clone` would all
have to exclude it, and a refreshed token would otherwise read as a different
identity.

**Alternatives rejected:**
- Expiry on `principal.Principal`: see point 6. It also puts transport detail
on a domain type that jobs serialize.
- Re-verifying the token periodically inside the stream: the stream holds no
raw token after the gate, and keeping one in memory to re-verify only re-checks
signature and `exp`, which the timer already covers.
- Ending at expiry minus a margin: the client cannot refresh the token itself
(the proxy does), so an early end gains nothing and causes earlier reconnects.

**Files to modify:**
- internal/jwtauth/verifier.go (+ assertion_test.go)
- cmd/rela-server/main.go adapter (+ main_identity_test.go)
- internal/dataentry/router.go (`AssertedIdentity`)
- internal/dataentry/jwtgate.go (set the expiry), new small helper in the same
file or a `credexpiry.go`
- internal/dataentry/watcher.go (`runSSELoop`)
- Tests: internal/dataentry/jwtgate_test.go, sse_resume_test.go or a new
sse_expiry_test.go, live_test.go
- docs: GUIDE for the JWT gate / live views (see Documentation Planning)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `exp` comes from a signature-verified assertion. It is used only after
`ParseWithClaims` succeeds, and golang-jwt has already rejected a past or
missing `exp`. A far-future `exp` is the IdP's statement; rela keeps the stream
for that long, as it would accept the token for that long.

**Security-Sensitive Operations:**
- The change narrows access: it only ends streams sooner. An attacker cannot
extend a stream, because the expiry is read from the verified claims only and
nothing else writes the context key.
- No new error path or log line exposes token content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria; each names its test. The SSE
tests drive the real router with the JWT gate and a stub verifier inside a
`synctest` bubble, so they cover gate, context and loop together. The stub
verifier gets a per-call identity (a func or a token-to-identity map) so AC5 can
give the stream and the PUT different expiries. A reflection test in
cmd/rela-server checks that the adapter sets every `AssertedIdentity` field,
since a forgotten field is the bug TKT-OJL2GN and TKT-IAC8TX fixed.

**Edge Cases:**
- Expiry in the past or equal to now: the timer fires at once; the stream ends.
- Zero expiry: no timer; the stream behaves as today.
- Expiry far in the future: one timer, no busy loop.
- Expiry fires while a flush is armed: the stream ends without writing the
pending frames; the client resumes from its last event id.
- Legacy `/api/events` stream: ends at expiry too.

**Negative Tests:**
- A stream without the gate does not end (criterion 2).
- An invalid or expired token still gets the gate's 401 (existing tests).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Short-lived tokens (for example 5 minutes) make streams reconnect that
often. Each reconnect resumes from the last event id, re-PUTs the live-view
subscriptions, refetches every subscribed GET in the client, and makes the
server run one baseline replay per subscription. Tabs that share one token all
reconnect at the same moment. No change is lost; the cost is one baseline round
per tab per token lifetime, which the per-identity subscription cap bounds. No
jitter: ending early only moves the burst, and the client cannot get a fresh
token before the proxy issues one.
- A proxy that does not refresh the assertion on the reconnect request gives
the client a 401 loop with backoff up to 30 s. That is the right outcome for an
expired session; the page already shows the stream as disconnected.
- Timing tests can flake: the expiry tests use `testing/synctest` (Go 1.26)
with an in-process ResponseWriter, so the fake clock decides when the timer
fires and "does not end" is checked without real waiting.

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- GUIDE on the JWT gate / identity assertion (docs-project): one paragraph that
event streams end at the assertion's expiry and the client reconnects.
- GUIDE-realtime live views section: the same note.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none critical or significant. Minor and nit, all
addressed in this plan: RR-V0FK61 (AC3 check before register), RR-JVPBF5 (cap
the write deadline at the expiry), RR-XSF73A (MCP watch out of scope), RR-98RGLX
(reconnect cost in Risks), RR-D9HA5C (`ExpiresAt` includes leeway), RR-XXFUBG
(synctest), RR-W007E9 (per-call stub verifier), RR-C16SOF (adapter reflection
test).
