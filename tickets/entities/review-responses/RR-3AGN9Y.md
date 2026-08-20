---
id: RR-3AGN9Y
type: review-response
title: Server-side GC sweep has no audit sink in the plan's wiring
finding: The plan sources audit from writeServices.Audit — a CLI bundle. The GC sweep also runs as a server-lifecycle goroutine wired in appbuild, where that bundle doesn't exist. The audit.Audit sink must be injected into the GC engine at the appbuild wiring site (same sink the entitymanager audit hook uses), or server-side GC deletions would be silently unaudited.
severity: minor
resolution: 'Amendment A8: the GC engine takes the audit sink as a nil-rejected constructor dependency (per the constructors-reject-nil rule), injected at the appbuild wiring site with the same audit.Audit the entitymanager hook uses; writeServices.Audit is just the CLI''s route to that sink.'
status: addressed
---
