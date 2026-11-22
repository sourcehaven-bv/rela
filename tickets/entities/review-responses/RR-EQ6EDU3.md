---
id: RR-EQ6EDU3
type: review-response
title: 'No load check of the relation target type'
finding: 'Nothing checks at load that relate.type admits the user entity type or that one exists.'
reason: 'A load check would need acl.yaml in the trigger checks, which only see the schema. The guide states that a wrong type gives an automation error on each write.'
severity: minor
status: wont-fix
---
