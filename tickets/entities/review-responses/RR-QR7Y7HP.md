---
id: RR-QR7Y7HP
type: review-response
title: Local relation meta gating not handled
finding: visibility.Elements refuses a relation type whose meta the local policy can hide (ErrRelationMetaGated). The plan only handles the remote 422; a multi-type relation span would fail the run.
severity: significant
resolution: 'prepare() handles a local ErrRelationMetaGated: a multi-type span is split by type and a single type is skipped with side local. Tested in TestDiff_MetaGated/local.'
status: addressed
---
