---
id: RR-4KUPKV
type: review-response
title: Dispatch from a branch uploads to the wrong tag
finding: github.ref_name is the branch name on a workflow_dispatch from a branch, so gh release upload would target a non-existent release.
severity: significant
resolution: The release job outputs the tag from dist/metadata.json; the sbom job uploads to that tag.
status: addressed
---
