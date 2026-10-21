---
id: RR-2WH8F9
type: review-response
title: SBOM failure blocks the desktop builds
finding: With the SBOM steps inside the release job, a syft or download failure fails the job after the release is public, and desktop never runs.
severity: significant
resolution: SBOMs moved to a separate sbom job that needs only release, so desktop runs regardless.
status: addressed
---
