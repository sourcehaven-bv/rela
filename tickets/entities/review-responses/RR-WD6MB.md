---
id: RR-WD6MB
type: review-response
title: Schema-loading flicker is real, plan dismisses it without committing to a behaviour
finding: schemaStore.load() is async and not awaited by the router (frontend/src/router/index.ts has no schema gate). Cold deep-link to /document/foo/T-001 mounts DocumentView before schema is loaded → editFormId === undefined → button hidden → schema loads → button appears. Real layout shift in .header-right. EntityDetail has the same flicker; living with it is defensible. The plan should commit to a behaviour explicitly (accept the flicker, or render a hidden-visibility placeholder while schemaStore.loaded === false) instead of saying 'low risk'.
severity: minor
reason: Accepted as documented behaviour. Same flicker as EntityDetail.vue today; users tolerate it. Adding a placeholder slot adds complexity for ~100ms of layout stability. If we ever address it, do it once at the schema-store level for all views, not per-component.
status: wont-fix
---
