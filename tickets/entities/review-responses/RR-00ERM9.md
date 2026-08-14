---
id: RR-00ERM9
type: review-response
title: Automation-set properties bypass the field gate — correct, but the plan leaves it as an open question
finding: 'The plan raises but does not answer whether automation-set properties should be field-gated. They are not, and should not be — but leaving it open invites the next reviewer to ''re-litigate'' or, worse, ''fix'' it. Confirmed: manager.go:589-592 applies autoResult.PropertiesSet via e.SetString AFTER the caller''s diff, so a gate that ran on the caller''s set/unset never sees them. This is correct: the field gate implements affordance PARITY (write_handler.go:373 calls it ''reject writes that conflict with what the resolver would have surfaced on GET'') — it constrains what a PRINCIPAL may author, not what the SYSTEM may derive. Verdicts are principal-scoped, so gating automation output would mean a user who cannot author `status` could never trigger an automation that sets `status`, breaking every workflow automation in the project''s own metamodel.yaml. The precedent is exact: dataentry runs validateFieldWrite at write_handler.go:376 then hands off to UpdateEntity at write_handler.go:436, where automation freely sets whatever it likes — so automation already bypasses the field gate in the ONLY path that has one. The plan preserves the status quo; it just needs to say so in the godoc.'
severity: significant
resolution: |-
    ACCEPTED. Status quo preserved (automation-set properties are NOT field-gated), but the semantics move from 'unanswered plan question' to written contract on the FieldWriteGate godoc: 'The field gate constrains caller-authored property changes only. Automation-derived properties (automation.Result.PropertiesSet) are system writes and are deliberately NOT gated — the gate enforces affordance parity with what the resolver would surface on GET for this principal, and automation is the system acting, not the principal.'

    Pinned by a test: a principal who may not author `status` triggers an automation that sets `status`; assert the write succeeds and status is set. Without that test a future 'consistency' change silently breaks every status-transition automation in the project's own metamodel.yaml.
status: addressed
---

## Resolution

Write the semantics into `FieldWriteGate`'s godoc, not just the plan:

> The field gate constrains **caller-authored** property changes only.
> Automation-derived properties (`automation.Result.PropertiesSet`) are system
> writes and are deliberately **not** gated — the gate enforces affordance
> parity with what the resolver would surface on GET for this principal, and
> automation is the system acting, not the principal.

Add a test pinning it: a principal who may not author `status` triggers an
automation that sets `status`; assert the write succeeds and `status` is set.
Without that test, a future "consistency" change silently breaks every
status-transition automation.

## Evidence

- `internal/entitymanager/manager.go:589-592` — automation applies after the
caller's diff.
- `internal/dataentry/write_handler.go:373` — the gate's stated purpose is
affordance parity with GET.
- `internal/dataentry/write_handler.go:376` then `:436` — gate, then
`UpdateEntity`, inside which automation is ungated. Status quo confirmed.
