---
id: RR-D8NWM4
type: review-response
title: history:read is a global-only 'audit-everything-deleted' super-permission — document blast radius
finding: 'holdsPermission is global-only by design (resolver.go:218-233): a role granting history:read lets the holder read the deleted-history of EVERY entity of every type, incl. ones they never had live access to and whose existence they''d otherwise never learn. Deleted entities are the most likely to have been removed for sensitivity, so this is an information-disclosure amplifier (compounded by RR-YDMJV7 field leak and RR-A3RNT0 pre-access leak if unfixed). Acceptable as a shortcut (cascade-delete tears down the conferring relations that a finer per-entity gate would need — IDEA-CQMKMD), but must be documented in docs/acl-security.md as an audit super-permission, not shipped silently over-broad.'
severity: minor
resolution: 'Addressed by documentation: history:read is shipped global-only (matching holdsPermission''s design) and documented in docs/acl-security.md as an ''audit-everything-deleted'' super-permission with an explicit blast-radius note — not silently over-broad. Finer per-entity scoping is impractical because cascade-delete tears down the conferring relations (IDEA-CQMKMD), so global is the defensible shortcut.'
status: addressed
---
