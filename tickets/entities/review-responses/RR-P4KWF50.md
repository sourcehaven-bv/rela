---
id: RR-P4KWF50
type: review-response
title: Subtests accept any error
finding: Store-starts-empty and ACL-denial cases used require.Error.
severity: minor
resolution: Now require.ErrorIs with store.ErrNotFound and acl.ErrForbidden.
status: addressed
---
