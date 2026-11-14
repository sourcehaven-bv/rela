---
id: TKT-HB313W5
type: ticket
title: 'rela test: stub and record outgoing HTTP calls'
kind: enhancement
priority: medium
effort: m
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

`rela test` refuses every network call, so a script that calls an outside API (a
payment provider, an operator API, a provisioning endpoint) cannot be tested
without hand-written seams. This ticket adds HTTP stubs to the test framework: a
case declares which calls its code may make and what each returns, and then
inspects the calls that were made.

Scope:

- `t.http.stub{...}`: match on method, URL (exact, prefix or RE2 pattern),
headers, and the body (exact, JSON subset or form fields). Answer with a status,
headers and a body (`body`, `json` or a fixture `file`), with a simulated
`timeout` or `network` error, or with a sequence of answers.
- An unmatched call fails the case with the registered stubs listed. The
script still gets a network error, and a script that swallows it still fails the
case. Nothing reaches the network.
- Recorded calls: `stub:calls()`, `stub:count()` and `t.http.calls()` (all
calls in order, with method, url, headers, body, decoded json and the stub that
answered).
- `t.secrets{...}`: fake secret values for scripts and webhook signatures.
A case never reads `.rela/secrets.yaml`, so recorded requests and reports cannot
hold a real secret.
- Flows: job steps run under `rela test`. `actor:start_flow(name, ref)` starts
a run, `t.flows.work()` runs the queued attempts and due rounds on the case's
clock, and `t.flows.run(id)` reads the run and its job. Script jobs and `kind:
webhook` jobs both send through the stubs.

Out of scope: AI stubs (`t.ai.stub`), split out to its own ticket. The guide is
part 4 of TKT-0MCLX9S (TKT-E2IMTMG); the notes below are for it.

## Guide notes (for TKT-E2IMTMG)

Section "Stubbing HTTP calls":

```lua
local t = require("rela.test")

t.case("a 503 then a 201 retries the payment", function()
  t.secrets{mollie_key = "test_key"}
  local mollie = t.http.stub{
    name = "create payment", method = "POST",
    url = "https://api.mollie.com/v2/payments",
    headers = {Authorization = "Bearer test_key"},
    json = {amount = {currency = "EUR"}},
    responses = {
      {status = 503},
      {status = 201, file = "tests/fixtures/mollie/payment.json"},
    },
  }
  local carol = t.as("carol")
  local id = carol:start_flow("pay", carol:create("invoice", {}))
  t.expect(t.flows.work()).to_be(1)          -- the 503
  t.advance(t.seconds(30))                    -- the job's backoff
  t.expect(t.flows.work()).to_be(1)          -- the 201
  t.expect(t.flows.run(id).state).to_be("paid")
  t.expect(mollie:count()).to_be(2)
end)
```

Points the guide should make:

- The first registered stub that matches and is not spent answers. `respond`
answers every call; `responses` answers once each, in order.
- `url` ignores query order; `url_pattern` must match the whole URL.
- Header names in stubs and recorded calls are case-insensitive / canonical
(`Content-Type`). In the script, response headers stay lower case.
- A `file` path is relative to the project root, inside it, with no symlink.
- Job attempts never run on their own in a case: call `t.flows.work()`, and
move the clock past a retry's backoff first.
- AI calls are still refused.

The full reference is the godoc on `fileRun.module` in `internal/apptest`.
