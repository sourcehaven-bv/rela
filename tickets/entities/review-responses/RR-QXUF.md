---
id: RR-QXUF
type: review-response
title: Cross-role union under per-type opt-in was non-monotonic (more roles = less access)
finding: 'Original draft''s ''any role declaring fields: T opts T into closed-world for the user'' meant adding a role could SHRINK access: user with permissive role B (write: [ticket], no fields:) and restrictive role A (fields: {ticket: [status]}) gets closed-world for ticket, losing B''s implicit write access on all non-status fields. Surprising and unreasonable.'
severity: significant
resolution: 'Redefined as per-role-per-type opt-in evaluated independently and unioned: each role with fields: {T: [...]} is closed-world FOR THAT ROLE''s contribution. Roles without fields: contribute empty writability sets (don''t grant per-field, but don''t shrink other roles'' grants). Union per-role writable sets. Result: monotonic — more roles = strictly more or equal access. Added 4 cross-role union tests with mixed declared/undeclared fields: blocks.'
status: addressed
---
