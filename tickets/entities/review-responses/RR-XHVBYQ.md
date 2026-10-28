---
id: RR-XHVBYQ
type: review-response
title: ReplyTarget rule disagrees with the ownership rule and accepts placeholder authors
finding: 'sameAuthor compares user AND org (authz.go:363), and the Author doc says two orgs may share a user name. ReplyTarget compares the user only. Also, nothing skips a parent author of `unknown` (deployments without an identity source) or an empty user, so every reply there writes rows for `unknown`. Fix: decide on purpose and say so. Since rows name a user only, comparing the user alone is defensible, but document the cross-org case in the reply table. Have ReplyTarget return false for "", principal.Unknown and IsRelaIdentity authors. Add channelstest rows for these.'
severity: minor
resolution: ReplyTarget compares user only (documented, cross-org row in table) and returns false for empty, unknown and rela identities; channelstest rows added.
status: addressed
---
