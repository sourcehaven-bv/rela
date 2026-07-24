---
id: RR-L6UXCF
type: review-response
title: 'available_on is display-only: handleCommandExec never validates client-supplied entity_id/list_id/view_id against command scope'
finding: |-
    Answering the plan's implicit assumption that config-load validation (validate.go:87, validateCommands) covers request-time safety: it does not.

    resolveCommands/matchesPage (commands.go:47-108) run EXCLUSIVELY on the GET /api/v1/_commands render path. handleCommandExec (commands.go:281-341) switches on cmd.Context and reads entity_id / list_id / view_id straight from the query string. It never calls matchesPage and never compares the supplied ID against cmd.AvailableOn.

    VERIFIED at commands.go:302-311:

      case "entity":
          entityID := r.URL.Query().Get("entity_id")
          svc := h.services()
          entityDomain, err := svc.Store.GetEntity(r.Context(), entityID)
          if err != nil { http.Error(w, "Entity not found: "+entityID, 404); return }
          input = h.buildEntityInput(r.Context(), entityDomain)

    No check that entityDomain.Type is in cmd.AvailableOn.EntityTypes. Same omission for list (commands.go:313-321 — list_id checked only for existence in s.Cfg.Lists) and view (commands.go:323-335).

    FAILURE SCENARIO: command `redact-pii`, context:entity, available_on.entity_types:[customer], script reads RELA_ENTITY_ID (set at commands.go:677). UI only ever offers it on customer pages. Client POSTs /api/command/redact-pii?entity_id=TKT-72SCPR — the entity resolves, buildEntityInput runs, the script executes against a ticket it was never scoped to.

    This is PRE-EXISTING, not introduced here. But the plan's Security section cites validate.go:87 as though it establishes an enforcement boundary. validateCommands only checks that referenced views/lists/entity-types EXIST in config; it has nothing to do with request-time authorization. Citing it as the input-validation story is the plan's core analytical error.

    RECOMMENDED (high leverage, ~10 lines, in a file already being touched): call matchesPage in handleCommandExec with the supplied params and 403 on mismatch. This makes available_on an actual boundary and would make the plan's 'the server already authorizes these' claim become true rather than needing deletion.
severity: significant
resolution: |-
    Scoped to TKT-MJ02AO: call matchesPage in handleCommandExec with the client-supplied entity_id/list_id/view_id and 403 on mismatch, making available_on an actual boundary. If that enforcement is rejected during MJ02AO planning, the fallback obligation is to document in docs/data-entry.md that available_on is display scoping only — doing neither leaves a documented lie.

    RESOLVED (2026-07-24) via the documentation fallback, not the enforcement path. TKT-MJ02AO (merged PR #1180, develop commit 69034972) did NOT add exec-time matchesPage enforcement — that was deliberately deferred because scoping command payloads is a behavior change to what existing scripts receive. Instead the fallback obligation was discharged: docs/data-entry.md now states plainly that available_on is not an authorization boundary, and docs/acl-security.md's 'What a command permission actually confers' section documents that payloads are not read-gate scoped in any context. The enforcement work is tracked as TKT-2FDTJE (priority high).
reason: 'Deferred to TKT-MJ02AO, resolving PLAN-CNDJ78 blocking decision B. Justification: the fix is a backend change (a matchesPage call plus 403 in handleCommandExec) in the same file and review as the ACL guard, so bundling them is cheaper and more coherent than splitting a ~10-line enforcement change into a frontend ticket. Deferring is safe because the pre-existing exposure is unchanged by TKT-72SCPR — that ticket adds no new exec path and no new client-controlled ID — and RR-KNEF4K independently closes the widening risk by typing the modal prop as a closed union so a forged param cannot originate from a call site.'
status: addressed
---

## Minimum plan change

Add to PLAN-CNDJ78 Security Considerations, verbatim:

> `available_on` is **display scoping, not an authorization boundary**.
> `handleCommandExec` does not enforce it. Any client that knows a command
> ID can invoke it with an arbitrary `entity_id` / `list_id` / `view_id`.

## Optional but recommended

Enforce `matchesPage` at exec time. One call before the context switch at
`commands.go:301`. Collapses this finding and materially improves RR-65KG68. If
taken, it becomes a backend change and the ticket's "frontend-only apart from
tests" framing needs updating.
