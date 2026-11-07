---
id: RR-LLRDM02
type: review-response
title: Clock threading list incomplete
finding: 'memstore UpdatedAt/softdelete time.Now, lease SystemClock, audit/version stamps, template.go fallback, scopes.go evaluator: 17 files on the default clock.'
severity: significant
resolution: Plan lists each site as threaded or documented out of scope; memstore timestamps, lease clock, automation, predicate (incl. scopes), computed and Lua are threaded; audit/version stamps documented as wall clock. Guard test scans write-path packages for time.Now with an allowlist.
status: addressed
---
