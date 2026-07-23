---
id: RR-37AYC0
type: review-response
title: entity/list command payloads bypass the read gate, so a command:* grant is a read-everything oracle — contradicting the docs shipped in this ticket
finding: |-
    This ticket deferred `context: view` on the stated grounds that its payload is the whole traversal closure 'assembled without read-gate scoping', so a grant would confer read access wider than the entry entity. That reasoning is correct. The problem is it applies to the contexts that WERE shipped as grantable.

    VERIFIED:

    - `context: entity` — handleCommandExec calls svc.Store.GetEntity(r.Context(), entityID) directly (commands.go:387) with NO PermitsRead check, and entityID comes straight from the query string. relationsForEntity (commands.go:233) then loads every incident relation with store.DirectionBoth. So a principal holding one entity-command permission can point it at ANY entity id in the store and receive that entity plus all its relations on the script's stdin.
    - `context: list` — listFromStoreByTypes performs a raw ListEntities drain with no ReadQuery scoping, and list_id is caller-supplied, so the principal can select any configured list rather than the one whose page they are on.

    CONSEQUENCE: a single `command:*` grant is materially broader than 'may run this script'. It is closer to 'may read every entity of the selected shape', because the script receives the payload regardless of the principal's per-entity read verdicts.

    WHY THIS IS A DOCS DEFECT, NOT ONLY A CODE ONE: docs/acl-security.md as written in this ticket singles out view as the wide-blast-radius context and implies the others are narrow. An operator reading it would reasonably conclude that granting `command:export-ticket` is scoped to tickets the grantee can already read. It is not. The docs currently overstate the safety of the shipped contexts.

    MINIMUM ACTION (docs, no code): state plainly that a command permission confers read access to whatever the command's context assembles — for entity, any entity id the caller supplies; for list, any configured list — and that command payloads are NOT read-gate scoped in any context. That makes the view deferral a difference of degree rather than kind, which is what it actually is.

    FULL FIX (code, likely its own ticket): scope the payload builders through readGateFromContext — PermitsRead for the entity context, ReadQuery for the list context — and then reconsider whether the view deferral is still necessary or whether the same scoping generalizes.
severity: significant
resolution: |-
    FIXED at the docs level (the minimum action the finding specified); the code-level read-gate scoping is deferred with a reason.

    docs/acl-security.md gained a 'What a command permission actually confers' section stating plainly that command payloads are NOT read-gate scoped in ANY context, with a table showing exactly what each context hands the script and what scopes it:

      entity  → the entity at the caller-supplied entity_id + every incident relation | scoped by: nothing
      list    → every entity in the caller-supplied list_id, post-filter              | scoped by: nothing
      global  → project paths only                                                    | n/a
      view    → entry + entire traversal closure                                      | not grantable

    It now says explicitly: 'Treat a command grant as "may read every entity of this shape", and scope grants accordingly', and notes that entity_id/list_id come from the request rather than the page the user was on.

    The view paragraph was rewritten to stop implying view is categorically different: 'The difference from the contexts above is degree, not kind: a view's traversal closure is unbounded by the config an operator can read, so the blast radius of the grant is not knowable in advance.' That is the honest distinction and it preserves the deferral rationale without overstating the safety of what shipped.

    DEFERRED (code): routing the payload builders through readGateFromContext — PermitsRead for entity, ReadQuery for list. That is a behavior change to what existing commands receive on stdin (a command that today sees all entities of a type would start seeing a subset), so it can break working scripts and deserves its own ticket with a migration note. Documenting the current behavior accurately is the correct interim: an operator can now make an informed grant decision, which they could not with the previous text.
status: addressed
---

## Evidence

`internal/dataentry/commands.go:385-391`, entity context — no read gate:

```go
case "entity":
	entityID := r.URL.Query().Get("entity_id")
	svc := h.services()
	entityDomain, err := svc.Store.GetEntity(r.Context(), entityID)
	if err != nil { ... 404 ... }
	input = h.buildEntityInput(r.Context(), entityDomain)
```

Compare the read path, which *does* gate — `history_handler.go:114`
(`a.gateReadOrNotFound`) and the `PermitsRead` contract in `readgate.go`.

## Why I am filing this against my own change

The ticket's view-deferral rationale is sound, and I wrote it. But applying that
reasoning consistently means the shipped contexts have the same property to a
lesser degree, and the docs I wrote imply otherwise. Correcting the docs is
cheap and honest; leaving them is the kind of overstatement that gets an
operator to grant something they would not have.
