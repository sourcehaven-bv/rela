---
id: RR-QUMMHHZ
type: review-response
title: Without acl.yaml a peer key reads everything
finding: Under NopACL attachACLRequest is not installed, so a valid key read every entity although the docs say acl.yaml decides. Security review.
severity: minor
resolution: peerKeysFor refuses every peer key with 401 when the ACL is not declarative. TestPeerKey_NoACLRefusesEveryKey pins it; docs updated.
status: addressed
---
