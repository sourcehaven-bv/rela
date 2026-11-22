---
id: RR-FTQ9URJ
type: review-response
title: Header-only run is always inactive
finding: A header without engine state is shown as inactive even when its stored status is not terminal, unlike RunList.
severity: nit
reason: Predates this change and concerns a run that is still starting or orphaned; out of scope for the subject fix.
status: wont-fix
---
