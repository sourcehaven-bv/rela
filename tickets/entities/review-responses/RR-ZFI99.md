---
id: RR-ZFI99
type: review-response
title: ErrEncrypted breaks search index, validator, and other ListEntities consumers
finding: 'internal/search/index.go:332, internal/validator/validator.go:159, internal/lua/runtime.go:745, and several cli/*.go list paths consume ListEntities and bail-or-skip on error. Plan claims ''no behavior change'' but today these only see parse errors when the user explicitly opens an encrypted file; after this change every list-walk yields one ErrEncrypted per encrypted file. Search index will silently rebuild as partial. Validator will fail loudly on any project with an encrypted file, blocking ALL data-entry writes (every PATCH triggers validation). Plan must enumerate every iterator consumer and choose a uniform policy: either ListEntities skips encrypted entries (yielding them only via the new sibling iterator) or each consumer adopts skip-with-log.'
severity: critical
resolution: 'Resolved by design pivot: encrypted files no longer surface as ErrEncrypted from ListEntities. Instead, fsstore returns a normal *entity.Entity with empty Properties and Inaccessible field populated. Search index, validator, Lua iterator, and CLI list paths see a regular entity and process it. Validator change: rules whose target property is in e.Inaccessible are skipped with debug log, not error. This means no project-wide validation abort, no search-index partial rebuild, no cascade. Plan AC9 and AC11 cover this.'
status: addressed
---
