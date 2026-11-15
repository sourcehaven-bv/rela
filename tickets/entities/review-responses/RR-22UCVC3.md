---
id: RR-22UCVC3
type: review-response
title: entity_refs ignores unknown option keys
finding: A typo such as contents or id silently took the legacy all-types scan.
severity: minor
resolution: parseEntityRefsOpts calls rejectUnknownKeys for types/style/format/limit/ids/content. TestEntityRefs_IDs covers {contents = ...}.
status: addressed
---
