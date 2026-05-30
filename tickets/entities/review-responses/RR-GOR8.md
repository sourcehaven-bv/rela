---
id: RR-GOR8
type: review-response
title: Dry-run request struct duplicated from real create; future field additions will drift
finding: 'handleV1DryRunCreate defines its own anonymous request struct (id/prefix/properties/content) parallel to handleV1CreateEntity''s. If a future ticket adds a field to the create request (e.g. an option flag), the dry-run silently won''t accept it. Fix: extract a shared named struct (e.g. createRequestBody) referenced by both handlers, or at minimum a code comment that pins the contract divergence as intentional (relations are NOT accepted in dry-run by design).'
severity: nit
resolution: 'Added a comment on the dry-run request struct pinning the contract: dry-run mirrors the real create body minus `relations` (deferred by design), and a future field addition must update both structs together. Kept as comment rather than a shared named struct to avoid coupling two handlers with a wider type than either needs today (CLAUDE.md: don''t extract for its own sake).'
status: addressed
---
