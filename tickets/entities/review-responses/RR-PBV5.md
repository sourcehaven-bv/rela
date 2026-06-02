---
id: RR-PBV5
type: review-response
title: Replace font-awesome npm with @fortawesome/fontawesome-free
finding: 'font-awesome on npm is unmaintained; @fortawesome/fontawesome-free is the maintained successor and still ships FA4-compatible class names in its v4 line. Cost: one-line dep + import path. Benefit: active maintenance + supply-chain attestations.'
severity: nit
reason: font-awesome@4.7.0 is a leaf CSS+font asset (zero transitive deps, no JS execution surface) pinned to a specific version. No security risk to actively maintain. Migrating to @fortawesome/fontawesome-free@4.x would change the import path with no behavioral benefit. The risk this nit raises is purely 'this package is old' — true but not actionable. If EasyMDE ever ships its own toolbar icons we'll drop this dep entirely; until then, pinned is fine.
status: wont-fix
---
