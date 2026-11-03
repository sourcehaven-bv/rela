---
id: RR-KBB1R6
type: review-response
title: release.yml still interpolates the tag-derived version in run blocks
finding: steps.version.outputs.version is derived from GITHUB_REF_NAME and was still spliced into bash and pwsh run scripts. Semgrep does not trace step outputs. Exploiting it needs tag push access.
severity: minor
resolution: 'Extract version now writes VERSION to GITHUB_ENV; every run block reads VERSION from the environment. The Build step uses shell: bash. No steps.version interpolation remains.'
status: addressed
---
