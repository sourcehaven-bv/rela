---
id: RR-2PDNIPS
type: review-response
title: Detail and resolve accepted any path in the project root
finding: '[security] A file outside the entity directories, or in a directory that maps to no type, was judged only on the identity its own frontmatter claims.'
severity: significant
resolution: 'identifyConflict classifies by location: entity files must sit in a type directory under EntitiesDir, relation files directly under RelationsDir; anything else is unjudgeable and answers 404. Pinned by TestV1ConflictResolve_FileIdentity.'
status: addressed
---
