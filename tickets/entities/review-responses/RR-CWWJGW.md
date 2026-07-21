---
id: RR-CWWJGW
type: review-response
title: --read-only does not block command execution; this ticket puts a button on the dashboard
finding: |-
    Read-only mode is enforced exclusively through acl.ReadOnlyACL.AuthorizeWrite(WriteRequest) (internal/acl/readonly.go:21). Command exec never constructs a WriteRequest (see the sibling finding), so --read-only does NOT stop a command from mutating the project, writing entities, or running arbitrary shell.

    VERIFIED: internal/acl/readonly.go implements exactly one method, AuthorizeWrite. cmd/rela-server/main.go:125-126 wires it via appbuild.WithACL(acl.ReadOnlyACL{}). There is no HTTP-layer read-only middleware — grep for RELA_READ_ONLY/ReadOnlyACL across cmd/ and internal/cli/ shows wiring only at the ACL/entitymanager layer.

    Compounding: there is NO frontend read-only state whatsoever. grep for readOnly/read_only/readonly across frontend/src returns only CodeMirror editor attributes, form-field `readonly` config flags, and Vue's `readonly()` helper — nothing representing server read-only mode. The SPA hides Edit/Delete/+New in read-only mode because the SERVER omits those affordances, not because the client knows the mode. A command button rendered from GET /_commands has no such suppression: resolveCommands does not consult ACL, so the buttons will render and be clickable.

    e2e/tests/read-only-mode.spec.ts:8-11 already acknowledges the gap: 'Deferred phase-2 sites (Lua command buttons, ...) remain visible and 403 at the server on click.' The '403 at the server' half is TRUE ONLY for endpoints that build a WriteRequest. Command exec does not, so it does not 403 — it runs. The spec's stated rationale for excluding command buttons does not hold.

    FAILURE SCENARIO: operator runs `rela-server --read-only` to hand stakeholders a safe browsing link, reasonably believing read-only means read-only. A context:global command is configured. A stakeholder clicks the new dashboard button; mutating writes execute. The operator's mental model of --read-only is silently wrong, and the dashboard is the first page they land on.
severity: critical
resolution: 'Scoped to TKT-MJ02AO: blanket deny of command execution under ReadOnlyACL regardless of the permission: key, plus permission-filtered resolveCommands so buttons do not render. That ticket also corrects the inaccurate ''remain visible and 403 at the server on click'' comment at e2e/tests/read-only-mode.spec.ts:8, which is false for command exec today.'
reason: 'Deferred to TKT-MJ02AO, which resolves PLAN-CNDJ78 blocking decision A as option 1 (gate exec on ACL) rather than option 3 (document as operator-trust). Justification for deferring rather than fixing in place: closing this correctly requires the same permission machinery as RR-65KG68 plus a blanket ReadOnlyACL deny, and there is currently no frontend read-only state to gate buttons on — the SPA hides write controls only because the server omits affordances. Both belong in the backend prep PR. The risk is contained meanwhile: TKT-72SCPR depends-on TKT-MJ02AO, and its read-only acceptance criterion (''no command buttons render under --read-only'') cannot pass until this lands, so the dashboard button cannot ship ahead of the gate.'
status: deferred
---

## Evidence

`internal/acl/readonly.go:18-27` — the entire enforcement surface:

```go
type ReadOnlyACL struct{}

func (ReadOnlyACL) AuthorizeWrite(_ context.Context, _ WriteRequest) Decision {
	return Decision{
		Allow:    false,
		RuleKind: "read-only",
		RuleID:   "read-only-acl",
		Reason:   "this rela instance is configured read-only",
	}
}
```

One method. `WriteRequest`-shaped. Command exec builds none.

`e2e/tests/read-only-mode.spec.ts:8-11`:

```
 * Deferred phase-2 sites (Lua command buttons, settings / theme /
 * git, relation add/remove inside form widgets, inline-edit buttons
 * in related-entity cards) remain visible and 403 at the server on
 * click. The assertions below intentionally exclude those.
```

The exclusion is justified by an assumption ("403 at the server") that does not
hold for this endpoint.

## Decision required before implementation

Pick one and record it in the plan:

1. **Gate `handleCommandExec` on an ACL op** — correct, larger, arguably
its own ticket.
2. **Suppress command buttons AND 403 the exec endpoint under read-only**
— minimum viable; requires plumbing read-only state to the SPA, which does not
exist today.
3. **Explicitly document commands as an operator-trust surface outside
ACL**, and decide whether the dashboard mount is acceptable under that framing.

Option 3 minus the documentation is the current status quo, and it is what the
plan implicitly assumes without stating.
