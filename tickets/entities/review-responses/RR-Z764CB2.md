---
id: RR-Z764CB2
type: review-response
title: Scope wrongly said inbox entries serve previous
finding: InboxSession is copied field by field (realtime/inbox.go:332, apiwire InboxSession, inboxDecisionHead.ts) and would not carry previous.
severity: minor
resolution: 'Scope corrected: InboxSession does not carry previous; the inbox drawer follows the setting through OpenChannel.'
status: addressed
---
