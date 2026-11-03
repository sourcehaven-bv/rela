---
id: RR-8I07KN
type: review-response
title: '`channel edit -P` makes three separate service operations'
finding: "ChannelEditCmd.Run (internal/cli/channel.go:434-445) calls messageType, which is a Read; then messageProps calls propTypes, a second Read with Schemas; then Edit. Post makes two. Each Read is a new ACL request, config snapshot and channel-store lock. The plan (section 0) says a CLI command is one service operation, and the coercion pre-read is an accepted exception, but the two reads in edit are redundant.\tAn edit costs three store round trips and three access evaluations. On sqlite or postgres that is three lock or transaction cycles for one user action.\tUse one Read with BeforeSeq=seq+1, Limit=1 and Schemas=true. It returns the message's type and every type's schema, so edit coerces from one read. Fold messageType into propTypes, or return both from one helper."
severity: minor
resolution: Edit makes one Read (BeforeSeq=seq+1, Limit 1, Schemas) that gives the message type and its schema; messageType now reads from that view. Post keeps one read.
reason: Edit makes one Read (BeforeSeq=seq+1, Limit 1, Schemas) that gives the message type and its schema; messageType now reads from that view. Post keeps one read.
status: addressed
---
