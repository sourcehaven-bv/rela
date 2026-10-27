---
id: RR-RUL4TA
type: review-response
title: Read markers of a deleted user pass to the next holder of the same reader id
finding: 'internal/channels/reads.go:24 (Reader is a bare user+org string) with docs/realtime.md retention table ("A new user that gets the same id inherits only the read positions"). Markers outlive the user entity, so a new principal whose user id or raw login matches a deleted one sees, through GET .../_channels (read.seq), the inbox unread_channels (seq) and mark_read answers, which gated channels the previous person opened and how far they read. That is personal data of the previous user disclosed to another person. Content stays gated, so impact is limited, and ids minted as random entity ids rarely repeat; raw logins without user_entity_type (or reassigned email logins) can. Fix: fold into TKT-6V8K8I: erase a reader''s markers when the user entity is deleted (or on a forget-reader command) and state in the docs that id reuse without erasure transfers read positions.'
severity: minor
resolution: Documented in the realtime retention section and ACL guide that reusing a user id without erasure passes on read positions; erasure on user deletion added to TKT-6V8K8I
status: addressed
---
