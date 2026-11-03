---
id: RR-MVMV22
type: review-response
title: SBOMs are not listed in checksums.txt
finding: A consumer cannot check an SBOM against checksums.txt.
severity: minor
reason: The GitHub release API publishes a sha256 digest per asset, which bump-rela already uses. Adding them to checksums.txt would need the GoReleaser sboms stanza and syft in staging-build too.
status: wont-fix
---
