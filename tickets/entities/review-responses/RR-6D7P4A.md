---
id: RR-6D7P4A
type: review-response
title: 'Copy kernel authorized nothing for same-entity: a stranger could promote a draft they cannot read'
finding: |-
    VERIFIED by execution. My authorizeCopy returned nil for a same-entity copy whose definition had no `guard:`. The source read is elevated/raw, the target write was deliberately exempt from the ordinary write grant, so NO check of any kind ran:

      HOLE: stranger mallory promoted PAGE-1. created=true props=map[secret:classified title:Draft]

    Mallory held no role, could not read PAGE-1, could not write it — and published its draft including the field she may not see.

    The reasoning that produced this is worth recording because it was persuasive and wrong. §9.2 says a guarded state is writable only by copy definitions naming it as target, 'each carrying its own guard'. I read that as making the definition the authority and dropped the checks. But 'each carrying its own guard' is a CONDITION, and I had made the guard optional — so the sentence's premise was unmet and the conclusion did not hold. The reviewer's phrasing is exact: I deleted a check and the godoc rationalized it. The godoc was self-refuting too, citing 'the ordinary read path' as authorization when for same-entity that path is explicitly elevated and gates nothing.
severity: critical
resolution: |-
    Three checks, none redundant:

    1. READ on the source, ALWAYS (new Deps.CopyReadGate). Elevation decides which FIELDS travel, not whether the principal may touch the entity — without this, elevation becomes a way to read what you cannot read.
    2. The definition's guard, now MANDATORY AT LOAD for any definition targeting a non-default face (validateCopy), so §9.2's sentence always applies rather than being conditioned on operator diligence.
    3. WRITE on the target for CROSS-ENTITY only — new identity, new audience, so it is an ordinary create.

    The same-entity target stays exempt from (3) because nobody holds update on published by design; (1) and (2) stand in its place. Pinned by TestCopy_StrangerCannotPromote, verified to bite: removing the source read check reproduces the hole verbatim.
status: addressed
---
