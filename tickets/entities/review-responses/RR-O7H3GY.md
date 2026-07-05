---
id: RR-O7H3GY
type: review-response
title: A6 (requires_permission no role grants) can be an intentional lockdown, not a bug
finding: 'A6 flags a requires_permission naming a permission no role grants. But that''s a legitimate hardening pattern: an operator deliberately gating a relation behind a permission NOBODY holds locks the relation entirely (no one can write it). Flagging it as medium could be a false positive on intentional config. Downgrade A6 to low/info and word it as a question (''relation X is gated by permission Y which no role grants; no principal can write X — intended?'') rather than an error. Same softening applies to A7 (dead permission) which is already low.'
severity: minor
resolution: A6 downgraded to low/info, worded as a question ('no principal can write X — intended?') rather than an error, since a deliberately ungranted requires_permission is a valid lockdown pattern.
status: addressed
---
