---
id: RR-OE4GAV0
type: review-response
title: 'Design: field scope and redactor are separate collaborators'
finding: The stored-path decision (SearchFieldScope) and the completed view (HiddenProperties) could be wired from different resolvers; a NopRedactor beside a real scoper completes pending rows unredacted.
severity: significant
resolution: One ElementPolicy collaborator supplies SearchFieldScope and HiddenProperties and RelationMetaGated; PolicyRedactor implements it over one resolver.
status: addressed
---
