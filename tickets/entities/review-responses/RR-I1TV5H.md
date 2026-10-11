---
id: RR-I1TV5H
type: review-response
title: Link world can run ahead of the fetched mentions
finding: EntityDetail passed the live worldParam while mentions belong to the fetched viewData, so after a world switch old titles briefly linked into the new world.
severity: minor
resolution: EntityDetail records viewWorld at both fetch sites and passes it to renderMarkdown instead of worldParam.
status: addressed
---
