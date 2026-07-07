---
id: RR-B6NDXK
type: review-response
title: Dead/misleading markdown-content class on list-info regions
finding: 'EntityList.vue puts class="list-info ... markdown-content" on the rendered divs, but .markdown-content is only defined in EntityDetail.vue''s scoped styles — there is no global rule, so scoped styles don''t apply here and the class is inert. It misleads the next developer into assuming shared markdown typography that doesn''t exist. Fix: drop the inert class (the .list-info :deep() rules do the actual styling).'
severity: significant
resolution: Removed the inert `markdown-content` class from both list-info divs in EntityList.vue. The `.list-info :deep()` scoped rules provide the actual styling; verified the class no longer appears in the rebuilt ListView bundle.
status: addressed
---
