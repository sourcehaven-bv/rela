---
id: RR-L56D
type: review-response
title: tryExactPrefixMatch uses first-match instead of longest-match
finding: 'useBacktickAutocomplete.ts lines 276-298 `tryExactPrefixMatch` uses Array.find() to pick the first prefix that matches by exact or startsWith semantics. Iteration order is alphabetical (buildPrefixList sort). If two prefixes both pass the startsWith branch — e.g. project declares prefixes `FE` AND `FEAT-X` (yes, contrived but the metamodel permits it) and the user types `FEAT-X-`, both `FE+''-''` (''FE-'') is a prefix of ''FEAT-X-''? Actually no, but consider `FE` (no dash) AND `FEAT` (no dash) when user types `FEATURE-` quickly — ''FEATURE-''.startsWith(''FE-'') is false but ''FEATURE-''.startsWith(''FEAT-'') is true. OK FEAT wins. The deeper hazard: alphabetical order is deterministic but not semantically right — longest-prefix match is what the user expects. Today''s metamodel may not surface this, but the moment someone adds a prefix that is itself a prefix of another (`PROJ` and `PROJX`), behavior depends on alphabetical sort order. Recommend: sort matching candidates by `prefix.length` descending before returning. Same issue would exist if buildPrefixList''s collation changes.'
severity: significant
resolution: tryExactPrefixMatch now iterates all candidates and selects the LONGEST matching prefix instead of returning the first match. PROJ vs PROJX vs typed PROJX-... now disambiguates deterministically by length.
status: addressed
---
