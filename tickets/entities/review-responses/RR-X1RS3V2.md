---
id: RR-X1RS3V2
type: review-response
title: Use fsimport.Copy for fixtures instead of a new loader
finding: fsimport/copyinto.go copies a filesystem project (faces, relations, attachments, comments, migration record) into a store with no ACL/automations/validation.
severity: significant
resolution: 'Fixtures load via fsimport Copy into the case''s memstore with tool rela-test. Documented: computed values not materialized, unique not checked, no version rows, fixtures must be in the current schema shape.'
status: addressed
---
