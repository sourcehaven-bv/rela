---
id: BUG-X1479P
type: bug
title: Type-mismatched relation creates skipped the audit log
description: 'writeCreateRelation wrote a relation whose target type the allowlist refuses (DEC-HWZHA soft condition) directly to the store after Manager.CreateRelation rejected it. ACL ran first, but no audit record, version attribution, relation template or managed order was applied. GitHub #1806.'
priority: medium
effort: s
why1: writeCreateRelation wrote the relation directly to the store when the manager returned a type-allowlist error, and the store does not audit.
why2: Manager.CreateRelation could only reject a mismatch; it had no way to accept a soft condition, so the write-with-warning policy (DEC-HWZHA) was implemented beside the manager.
why3: When BUG-K6FEVB moved the ACL ahead of the fallback, the review weighed authorization only; audit, attribution and templates were not on the checklist.
why4: The rule that all writes go through entitymanager is stated in prose, and nothing flags a store write from a dataentry write path.
why5: Soft-condition policy lived in the caller instead of being an option of the single write path.
prevention: TolerateTypeMismatch makes the soft condition a manager option. TestTypeMismatchRelationWrite_Audited asserts the audit record for the soft path.
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

## Description

GitHub #1806 (IB review of #1803, CONTROL-8-15 logging).

A relation create to a target type the relation type does not allow is a soft
condition (DEC-HWZHA): the data-entry API writes it and returns a warning. It
did so by writing straight to the store after the manager refused, which skipped
the audit record (`OpCreateRelation`), version attribution, the relation
template, managed ordering and conflict mapping.

Fix: `entity.RelationOptions.TolerateTypeMismatch` lets the manager accept the
mismatch on its normal path, so the write is audited like any other.
`writeUpdateRelation` had the same fallback, which could never run; it is
removed. PR #1803 `writeBoundedFallback` should use the same flag.
