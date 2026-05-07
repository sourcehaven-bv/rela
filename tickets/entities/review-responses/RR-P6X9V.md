---
id: RR-P6X9V
type: review-response
title: Propagated changes don't go through no-op suppression
finding: |-
    When primary edge IS a real change, every propagated counterparty add gets emitted unconditionally — even if the back-edge in the live graph already matches byte-for-byte. Writes are idempotent so semantics don't break, but unnecessary disk write + spurious entity:updated SSE event for the counterparty. Auto-save SSE fan-out the plan was specifically designed to suppress (decision #11) leaks here.

    For inverses with separate intended properties: propagation OVERWRITES the existing inverse edge's properties with the primary's — destroying any per-edge customization (compounds with C2's aliasing).

    Fix: gate propagated counterparty write on a no-op check too. After computing back, check tx.GetRelation(back.From, back.Type, back.To); skip if equal.
severity: significant
resolution: propagateRelations now uses stageAdd/stageRemove helper closures that consult tx.GetRelation against the live graph; if the back-edge already matches the desired state byte-for-byte (relationsEqual + content equality), the propagated change is skipped. Counterparties no longer get spurious entity:updated SSE events on no-op back-edges.
status: addressed
---
