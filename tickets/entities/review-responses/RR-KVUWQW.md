---
id: RR-KVUWQW
type: review-response
title: Denials on the task-item path are not audited
finding: The relation-affordance and source field-read refusals return a ForbiddenError without an OpDeniedWrite record, unlike App.denyAffordance on the hand path.
severity: minor
resolution: Field-read and relation-affordance denials now go through a deny helper that records a denied-write audit row, as the hand-made relation path does. Probes (offers) are not audited.
status: addressed
---
