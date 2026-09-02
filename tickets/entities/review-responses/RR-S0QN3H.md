---
id: RR-S0QN3H
type: review-response
title: No array indexing in {{body.*}} — Alertmanager and modern Grafana cannot be ingested at all
finding: 'Interpolation cannot index into JSON arrays, so the two most common monitoring webhook formats are unusable without preprocessing. Measured through the production router: {{body.alerts.0.labels.alertname}} on a real Alertmanager v4 payload -> "" (empty, SILENTLY); {{body.evalMatches.0.metric}} on a legacy Grafana payload -> "". Nested OBJECTS work ({{body.commonLabels.severity}} -> "critical") and top-level scalars work ({{body.status}} -> "firing"), so the gap is specifically array element access. This blocks: Prometheus Alertmanager (all alert content lives in alerts[], with labels/annotations/fingerprint/startsAt per element) and Grafana 9+ unified alerting (Alertmanager-compatible, same shape). Grafana legacy (8 and earlier) is equally blocked via evalMatches[]. Icinga 2 is UNAFFECTED because an operator hand-writes the body in a NotificationCommand and would naturally emit a flat object. Worse than a missing feature, the failure is SILENT: an unresolved reference becomes the empty string by design, so an operator wiring up Alertmanager gets entities created with empty titles rather than an error, and the misconfiguration is only visible by inspecting stored data. Second, related finding measured at the same time: large integers lose precision through float64 (1756713600123456789 -> 1756713600123456768), which matters for epoch-nanosecond timestamps and 64-bit ids. Also note batched deliveries: Alertmanager groups MULTIPLE alerts into one POST, and the pipeline creates/updates at most ONE entity per delivery — so even with array indexing, alerts[1..n] would be dropped. Fix options: (a) support numeric path segments in interpolation and add a per-element iteration step, (b) document plainly that array-shaped producers need the Lua escape hatch (TKT-EFMRQM) or an external transform, (c) at minimum make an unresolvable path a load-time or request-time error rather than silent empty. Given the ticket''s motivating use case is monitoring alerts, shipping without (a) or an explicit (b) leaves the headline integration unsupported.'
severity: significant
resolution: 'Deferred to TKT-ZEACWJ (array indexing and per-element iteration), raised with the full measurement table and the blocker identified. Not fixed here because the right vehicle is internal/predicate — rela''s existing sandboxed Lua-expression subset — rather than bolting a second expression dialect onto the substitution-only {{...}} syntax. predicate already has ListType, NewList and a distinct Int type (which also fixes the float64 precision loss measured here), but walkAttrGet requires a string-literal key on a RecordType and explicitly rejects computed keys, so numeric indexing needs a new walk case and IR node. That is a grammar change to an engine shared by ACL affordances, state-machine transitions, automation conditions, wizard-form lint and validation rules, and the package doc requires an accepted IR node to keep identical semantics across every evaluator and future SQL target — it needs its own design review, not a webhook PR riding along. Batching is folded into the same ticket rather than split: Alertmanager sends N alerts per POST and the pipeline handles at most one entity per delivery, so indexing alone would still drop alerts[1..n]. Interim position documented in docs/webhooks.md: Icinga works today (operators hand-write a flat body); Alertmanager and Grafana need the Lua escape hatch or an external transform until TKT-ZEACWJ lands.'
status: deferred
---

Measured directly through `app.NewRouter()` with real payload shapes:

| Template | Result |
| --- | --- |
| `{{body.alerts.0.labels.alertname}}` (Alertmanager v4) | `""` — **silent miss** |
| `{{body.evalMatches.0.metric}}` (Grafana legacy) | `""` — **silent miss** |
| `{{body.commonLabels.severity}}` | `"critical"` ✅ |
| `{{body.status}}` | `"firing"` ✅ |
| `{{body.n}}` where n = 1756713600123456789 | `"1756713600123456768"` — precision lost |
| `{{body.b}}` / `{{body.z}}` / `{{body.o}}` | `"true"` / `""` / `{"a":1}` |

**Who this blocks**

- **Prometheus Alertmanager** — all content is in `alerts[]`. Blocked.
- **Grafana 9+ unified alerting** — Alertmanager-compatible. Blocked.
- **Grafana ≤8 legacy** — `evalMatches[]`. Blocked.
- **Icinga 2** — unaffected; the operator hand-writes the body in a
NotificationCommand and would emit a flat object.

**Why it is worse than a missing feature**: an unresolved reference becomes the
empty string *by design*, so an operator wiring up Alertmanager gets entities
with empty titles rather than an error. The misconfiguration surfaces only by
inspecting stored data.

**Batching compounds it**: Alertmanager groups multiple alerts into one POST,
and the pipeline creates/updates at most one entity per delivery — so even with
array indexing, `alerts[1..n]` would be silently dropped.

Given the ticket's motivating use case is monitoring alerts, shipping without
either array support or an explicit documented limitation leaves the headline
integration unsupported.
