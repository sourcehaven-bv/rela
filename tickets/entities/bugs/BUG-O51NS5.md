---
id: BUG-O51NS5
type: bug
title: Data-entry answers 500 instead of 401 when a handler finds no bound identity
description: Below the request edge, code fails closed with acl.ErrNoIdentity. The data-entry error mappers turn that into 500 (PATCH acl_query_failed and DELETE delete_failed) or a 403 (POST, whose AuthorizeWrite decision drops the sentinel). The answer should be 401 from one shared mapping. A mismatched identity stays 500.
priority: medium
effort: s
why1: The data-entry error mappers have no case for acl.ErrNoIdentity, so writeGateError and writeInternalError answer 500 and writeForbiddenIfACLDenied answers 403.
why2: Each mapper was written for the errors its call sites expected (store, gate query, ACL deny). The identity sentinels arrived later, when RequestFrom started failing closed below the edge.
why3: acl.Decision carries only text, so a write denied for a missing identity becomes a ForbiddenError that errors.Is cannot tell apart from a role deny.
why4: There is no single error-to-status function in dataentry. Status mapping is spread over several helpers, so a new sentinel must be added to each one.
why5: No test drives the handlers without a bound identity and asserts the status, so the gap stayed invisible behind the edge middleware that always binds one in production.
prevention: writeNoIdentity is the one place that maps ErrNoIdentity, and every shared mapper calls it. Decision.Cause keeps the sentinel through ForbiddenError. AM-no-identity-answers-401 pins read, list and write endpoints.
started: "2026-10-28"
completed: "2026-10-28"
status: done
---
