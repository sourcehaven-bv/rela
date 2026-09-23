---
id: RR-QOA8GX
type: review-response
title: TypeScript NavigationEntry lacks the entities fields
finding: /_config now serves entities query_scope and sort on navigation entries but frontend/src/types/config.ts did not declare them.
severity: nit
resolution: Added the three optional fields with doc comments.
status: addressed
---
