---
id: RR-UC90C0
type: review-response
title: Replay share key derived from the audit JSON format
finding: 'Architecture review: fingerprint hashed principal.MarshalJSON, the audit wire format, so dropping a field from audit output would merge principals Equal tells apart. sameIdentity was a second identity definition in dataentry.'
severity: significant
resolution: principal.Principal.ShareKey (own encoding over Equal's fields) and SameIdentity live in internal/principal. TestShareKey_MatchesEqual asserts the key is equal exactly when Equal holds, per field and for nil versus empty slices.
status: addressed
---
