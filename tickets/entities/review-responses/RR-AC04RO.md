---
id: RR-AC04RO
type: review-response
title: create row hides a lookup failure
finding: 'internal/realtime/live_subscription_test.go:209-222: the key func returns channels.Key{} when List fails or finds a number of channels other than one, so the failure shows up as a confusing key mismatch. Fix: pass t into the key func and require.NoError and require.Len there.'
severity: nit
resolution: 'Fixed: the create row key func takes t and uses require.NoError and require.Len; all key funcs take t.'
status: addressed
---
