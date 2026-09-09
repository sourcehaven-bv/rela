---
id: RR-DPYJ3V
type: review-response
title: The nolint reason on AllowAllCopyVisibility.Get cited a rationale that does not hold for it
finding: 'Get maps a store error to a miss with a //nolint:nilerr reading ''absent and denied are indistinguishable, as in every read gate''. That rationale is wrong for this type specifically: AllowAllCopyVisibility denies nothing, so there is no confidentiality property to preserve. The real reason is shape-consistency with appbuild''s copyVisibility so the caller''s !ok handling is uniform. Propagating a rationale that does not apply teaches the next reader something false. Separately, both this and the production gate turn a transient store failure into ErrCopySourceMissing, so an operator debugging a failed copy is told the source is missing for an entity that plainly exists — a pre-existing wart, flagged not fixed.'
severity: nit
resolution: Rewrote the nolint reason and the method doc to state the actual justification (match copyVisibility's error shape) and to say explicitly that the absent-and-denied-are-indistinguishable rationale does NOT apply here because this gate denies nothing. The shared wart is named in the same comment so it is discoverable from either site. Also documented that entityType is ignored because allow-all draws no per-type distinction (the reviewer's separate nit).
status: addressed
---
