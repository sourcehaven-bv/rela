---
id: RR-UD2E
type: review-response
title: Cards/list InaccessibleField silently swallows the reason
finding: |
  The HTML comment correctly notes inaccessibleByName is keyed on the entry, not per-card-entity. So cards/list always render the generic "inaccessible" tooltip. Right call for now -- the wire shape doesn't have the data -- but if git-crypt encryption is the primary inaccessibility case, users will see the generic tooltip and not know to run `git-crypt unlock`.
severity: minor
status: deferred
reason: |
  Requires backend wire-shape change: ViewEntity needs an inaccessibleByProperty map (mirror of the entry-level inaccessibleByName) so per-card fields can carry their own reason. That's an additive API change worth its own ticket with proper design + migration consideration. The HTML comment in EntityDetail now points at this RR-UD2E follow-up. Today's behaviour (generic tooltip) is correct for unknown inaccessibility reasons -- the regression is only against git-crypt-aware tooltips on cards/list, which is a narrow case. Not blocking.
---
