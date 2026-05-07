---
id: RR-FMLU1
type: review-response
title: Symmetric/inverse relation diff semantics under replacement are unspecified
finding: |-
    Plan acknowledges symmetric/inverse exist but ducks the actual semantics. Hard cases not resolved: (a) tagged is symmetric, A.tagged → [B,C]. PATCH A.tagged: [B]. Do we remove A→C only (graph inconsistent: C still has tagged→A) or both A→C and C→A (mutates C as a side effect)? (b) PATCH A.tagged: []. Do we delete back-edges from B,C? What about B's unrelated tagged→D? (c) Inverse relations: A.assesses→B, B.assessed-by→A. PATCH A.assesses: []. Does inverse on B get cleaned?

    Fix: add Symmetric/inverse subsection to Decisions:
    - For Symmetric:true or non-nil Inverse, every add/remove also stages the inverse edge. Diff is per-source-entity-and-relation-type; counterparties' unrelated edges are untouched.
    - A PATCH on A's symmetric relation MAY mutate B's, C's relation files. Document in API ref.
    - The single entity:updated event for A doesn't cover B,C; either also broadcast for affected counterparties, or document that observers rely on file-watcher reload.

    Add AC: symmetric add/remove updates both endpoints; counterparty's unrelated edges untouched.
severity: critical
resolution: 'Decision #7: symmetric/inverse propagation. Approach step 8 stages counterparty add/remove. ACs #14, #15, #16 cover graph state, event count, and inverse-named edges. Counterparty unrelated edges explicitly NEVER touched. Per user direction (Option 1a).'
status: addressed
---
