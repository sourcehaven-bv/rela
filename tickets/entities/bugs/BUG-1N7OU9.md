---
id: BUG-1N7OU9
type: bug
title: 'Attachment upload and delete ignore the fields: write policy'
description: 'The attachment write preflight checked visibility and update on the face but not the fields: policy. A principal with update could upload to or detach from a file field that fields: makes read-only (GitHub #1760 part 3).'
priority: medium
effort: xs
why1: attachmentWritePreflight checked visibility and the update grant but never called validateFieldWrite.
why2: The attachment endpoints were added as their own write path, beside PATCH and PUT, and copied only the ACL half of their checks.
why3: 'The fields: policy is enforced per handler in dataentry rather than at one point every write passes through.'
why4: No test asserts that every write endpoint for a property honors the same field verdict that the entity response advertises in _fields.
why5: 'fields: enforcement has no single choke point and no contract test across write endpoints, so a new endpoint can skip it silently.'
prevention: TestAttachmentWrite_HonorsReadOnlyField pins the upload and delete check. A contract test that checks _fields.writable against every property write endpoint, as affordances_contract_test.go does for _actions, would catch the next endpoint.
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

## Description

`PUT|POST|DELETE /api/v1/{plural}/{id}/_attachments/{property}` authorize
through `attachmentWritePreflight`
(`internal/dataentry/handlers_attachment.go`). It checked that the property is
visible and that the principal holds `update` on the face. It did not apply the
`fields:` write policy that `PATCH` and `PUT` apply through
`validateFieldWrite`.

So a role with `update` could fill or clear a file field the policy marks
read-only. The entity response already reported `_fields.<property>.writable:
false`, so the UI and the API disagreed.

## Reproduction

`TestAttachmentWrite_HonorsReadOnlyField`: with `screenshot` read-only, an
upload returned 200 and replaced the file.

## Fix

The preflight calls `validateFieldWrite` for the property after the `update`
check and denies through `denyAffordance`, which audits the denial like a
`PATCH` denial. Upload and delete share the preflight, so both are covered.
Parts 1, 2 and 4 of #1760 are tracked separately.
