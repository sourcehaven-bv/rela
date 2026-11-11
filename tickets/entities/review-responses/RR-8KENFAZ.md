---
id: RR-8KENFAZ
type: review-response
title: Rename guard refuses for flows without outside_edits
finding: The guard refuses a rename while any run is stored, also for a flow that declares no outside_edits.
severity: minor
reason: 'Deliberate: the decision for this ticket is to refuse a rename while a run is active on the subject. A run keys its subject by id, so a rename separates any run from its subject, not only governed ones.'
status: wont-fix
---
