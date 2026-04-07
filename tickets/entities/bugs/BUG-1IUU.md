---
id: BUG-1IUU
type: bug
title: v0.9 release blocked by govulncheck failures
description: 'The v0.9 release pipeline failed at the Security job because govulncheck reported 7 vulnerabilities reachable from our code: GO-2025-4116 (x/crypto SSH agent DoS), GO-2025-3754/GO-2026-4550 (cloudflare/circl), GO-2026-4473 (go-git), GO-2026-4601/GO-2026-4602 (Go 1.24 stdlib), and GO-2026-4923 (bbolt index OOR, no upstream fix yet, reached transitively via blevesearch).'
priority: high
effort: xs
why1: govulncheck reported reachable vulnerabilities in dependencies and the Go stdlib used by the v0.9 release build.
why2: Several upstream patches landed (x/crypto v0.43+, circl v1.6.3, go-git v5.16.5, Go 1.25.8) but our go.mod and CI Go version had not been bumped.
why3: There is no scheduled cadence for upgrading transitive dependencies; we only react when govulncheck fails a release.
why4: Dependency upgrades only happen reactively at release time, not on a regular schedule, so vulnerabilities accumulate between releases.
why5: We rely on a single govulncheck step in the release pipeline as the only enforcement point, with no separate alerting on the develop branch and no workflow to auto-bump dependencies.
prevention: 'Added scripts/govulncheck-filtered.sh that runs in release and security workflows; ignores only documented unfixable OSVs and fails on everything else. Bumped Go toolchain to 1.25.8 and refreshed x/crypto, cloudflare/circl, and go-git to clear all known reachable vulnerabilities. Future cadence: subscribe to govulncheck weekly schedule (already in security.yml) so we catch new advisories before they block a release.'
status: done
---
