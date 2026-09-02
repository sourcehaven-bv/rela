---
id: TKT-EFMRQM
type: ticket
title: 'Request-scoped Lua actions: full request in, arbitrary response out'
kind: enhancement
priority: medium
effort: m
tags:
    - needs-design
status: backlog
---

Sequenced AFTER [[TKT-1EM4KL]] (declarative webhook routes). That ticket covers
the common mappings in config; this one is the **escape hatch** for everything
the declarative vocabulary cannot express — an odd payload shape, a response a
third party needs in a specific form, logic that isn't find/create/append.

An action is *almost* the right primitive for webhook-style integrations. Two
things are missing, one at each end: the script cannot see the **incoming
request**, and it cannot produce an **arbitrary response**. Both ends are
projected through fixed, SPA-shaped structs.

## The actual gap

A webhook is just a regular HTTP call, and rela already has the machinery around
it. `POST /api/v1/_action/{id}` (`internal/dataentry/actions.go`) is a
**generic, operator-declared HTTP -> Lua endpoint**: ACL-bounded, per-action
`capabilities:` gating (TKT-YH52OM), `writeMu` serialization. Authentication and
ACL context arrive with the request, established by the proxy in front of rela,
so the script runs in a bounded context it can trust. The graph-side work
already functions — a Lua action can query entities and `PatchEntity` to append
to a markdown body.

What is missing is the data, in both directions.

**Request in.** `handleV1Action` decodes the body into `v1ActionRequest`, which
has exactly two fields — `entity_id` and `entity_type` (and `entity_type` is
deliberately ignored). Everything else is dropped by `json.Decode`. Query
parameters and headers are never read. `script.ExecuteAction` takes `params
map[string]string`, so there is no structured-payload channel even in principle.

**Response out.** A script's return value becomes
`script.ActionResponse{Redirect, Message, MessageType}` — a SPA-toast
vocabulary. There is no status code, no response body, no content type. A script
cannot answer `200 {"status":"ok"}`, cannot signal a retry with a 5xx, cannot
return anything a third-party caller would parse.

The IdP webhook receiver (`internal/dataentry/webhook.go`) has the same limit —
four fixed claims in, body discarded. It is not the thing to generalize; the
*action endpoint* already is the general path.

## Scope

Make an action **request-scoped**: the whole request in, an arbitrary response
out.

Request:

- **Parsed/raw body**, not a two-field projection. JSON reaches Lua as a table;
a non-JSON body is reachable as a string.
- **Query parameters** and a **safe subset of headers**.
- A structured-payload channel into `script.ExecuteAction`.

Response:

- **Status code**, **body**, and **content type** settable from Lua.
- The existing `{redirect, message, message_type}` shape stays the default, so
every current SPA action keeps working unchanged. The richer form is opt-in — a
script that returns today's shape must behave exactly as today.

Also worth deciding: a **declarative route** (config-declared path -> action) so
a third party gets a stable `/hooks/icinga` rather than `/api/v1/_action/...`.
Same mechanism; a naming/ergonomics question.

Preserve: `entity_id` resolution through `visibility.ScriptReader` (the gate
that stops this endpoint becoming a read bypass — `BUG-ZWTDH9`), per-action
`capabilities:`, and `writeMu`.

## Motivating use case

Icinga POSTs a JSON alert. A Lua action reads host/service/state from the body,
looks up the matching `incident` entity, appends the notification to its
markdown body or creates one, and answers with a status code Icinga can act on.

Today the script can see none of the request and can answer none of that.

## Design questions

- **Response is now attacker-influenced output.** A script-controlled body and
content type is an XSS/content-sniffing surface if it can be fetched from a
browser. The export path already solved this shape — `nosniff`, sandbox CSP,
`no-store`, sanitized `Content-Disposition` — and the same reasoning likely
applies. Whether an arbitrary content type is allowed at all, or an allowlist,
needs deciding.
- **Header exposure is an allowlist, not pass-through** — request headers carry
cookies, bearer tokens and proxy assertions that must not enter script scope.
- **Body size cap.** The action endpoint has none today; the webhook path caps
at 64 KiB sized for a JWT. A parsed body becomes a Lua table, so a cap now
bounds script-visible memory.
- **Idempotency.** A third-party producer retries. Whether rela offers a dedup
key or leaves it to the script is worth deciding once — the failure mode (an
incident body appended twice) is quiet.
- **Error mapping.** A script error currently becomes a rela error envelope.
With script-set status codes, which wins needs to be explicit.

## Out of scope

- Any producer authentication scheme in rela. The proxy in front (Pratique,
oauth2-proxy) terminates that and hands rela an ACL-bounded request.
- Outbound webhooks (rela calling out on graph change).
- Any Icinga-specific Go code — that mapping belongs in a Lua script, as
`examples/idp-sync.lua` is for the IdP case.
