---
id: RR-8W0UVY
type: review-response
title: EntityDetail does not pin passing the world
finding: Only markdown.test.ts covered the option; nothing checked EntityDetail passes it.
severity: nit
resolution: Added an EntityDetail.world.test.ts case for ?world=published and for no world; mutation-checked (fails when the world is dropped).
status: addressed
---
