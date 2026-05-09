---
id: RR-8LLD0
type: review-response
title: Sibling iterators add interface bloat and snapshot-divergence risk; prefer typed error
finding: 'ListInaccessibleEntities/ListInaccessibleRelations on the Store interface forces every backend (memstore, future postgres/sqlite) to implement methods that mean ''empty'' for non-fsstore backends. Data-entry must call the store twice and merge; merge must preserve sort order, dedupe, and reconcile snapshot divergence between the two calls. The rejection of typed-error approach is weak — every ListEntities consumer already inspects err. Reconsider: yield encrypted files as a typed error (*EncryptedError{Path, ID, Type}) in existing ListEntities/ListRelations; consumers ignore via errors.Is, data-entry uses errors.As to surface. One mental model, one snapshot. Or, if we keep sibling iterators, declare them on a fsstore-specific extension interface (per CLAUDE.md ''interfaces at the call site'') so non-fsstore backends don''t have to implement.'
severity: significant
resolution: Resolved by abandoning the sibling iterator approach entirely. Inaccessible is a field on entity.Entity itself, not a separate stream. ListEntities/ListRelations yield normal entities; consumers that don't care just use Properties as before. The save-path argument (PATCH needs redaction info to travel with the entity) is decisive — only field-on-entity satisfies it.
status: addressed
---
