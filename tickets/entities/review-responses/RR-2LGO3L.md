---
id: RR-2LGO3L
type: review-response
title: NopACL nil/empty scope map inverts to DenyAll-everything; needs explicit all-allow contract
finding: With 'absent = DenyAll', a nil map under NopACL means empty results for the no-ACL case (fail-closed inversion breaking AC9). The NopACL contract needs either a sentinel all-allow mechanism or guaranteed full enumeration of types including unknown ones.
severity: minor
resolution: 'Plan rev 2: reserved "*" wildcard scope entry. NopACL → {"*": AllowAll} (explicit all-allow, no enumeration needed, no fail-closed inversion). Lookup rule exact → "*" → DenyAll pinned in the seam godoc and conformance case 8.'
status: addressed
---
