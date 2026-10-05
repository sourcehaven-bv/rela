---
id: RR-RKV330
type: review-response
title: Stale docs still say a store satisfies the address reader surfaces
finding: visibility.UnrestrictedReader doc and dataentry/analyze.go comments claimed store.Store satisfies lua.EntityReader and analyzeReader structurally, and that NopACL reads are the raw store. After the GetAddress rename both are false.
severity: significant
resolution: Rewrote the UnrestrictedReader rationale (the compiler now refuses a bare store) and corrected the analyze.go comments (GetAddress naming; NopACL uses visibility.Unrestricted).
status: addressed
---
