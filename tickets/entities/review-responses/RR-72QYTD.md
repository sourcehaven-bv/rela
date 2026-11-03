---
id: RR-72QYTD
type: review-response
title: channelAliasesFor documents tombs as required but does not enforce it
finding: appbuild/channels.go:100 says "tombs is required with cs" but accepts nil, and EntityDeleted then panics in tombs.confirm. Per the constructor rule, return an error on nil tombs.
severity: minor
resolution: channelAliasesFor now returns an error when the store or tombs is nil.
status: addressed
---
