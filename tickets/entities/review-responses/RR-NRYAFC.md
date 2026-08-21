---
id: RR-NRYAFC
type: review-response
title: Migration singletons carry mutable metamodel state in a global registry
finding: Register(&FormRelationDirectionMigration{}) puts a single mutable instance in a package-level registry, and DetectFromNodeWithMetamodel mutates it via SetMetamodel on every call. Two projects migrated concurrently (or parallel tests) could infer against the wrong schema — and for this migration specifically, a stale m.meta writes WRONG DIRECTIONS into a user's config file. The race detector is on in CI, so it would surface eventually.
severity: significant
reason: 'Pre-existing pattern, not introduced here: DataEntryCleanupMigration and every other MetamodelAware migration share it, and fixing it means reworking the registry contract for all of them. Out of scope for this ticket, which is already a breaking change. Filed as follow-up TKT-E4I6NX so the specific new risk (a stale metamodel writing wrong directions into operator config, rather than merely a wrong detect) is recorded rather than lost.'
status: deferred
---
