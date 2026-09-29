---
id: RR-JH5WPS
type: review-response
title: FilterRelationsStrict widens the Reader interface
finding: Only ScriptReader calls FilterRelationsStrict, yet every Reader must implement it.
severity: minor
reason: Reader has two implementations (PolicyReader, AllowAllReader). The strict filter is FilterRelations with a different fault policy, so one contract keeps them from drifting.
status: wont-fix
---
