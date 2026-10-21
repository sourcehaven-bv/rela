---
id: RR-7EW47J
type: review-response
title: conflict paths are gated by git:sync rather than read access
finding: A git:sync holder who cannot read a type still sees its conflict paths, and the docs overstated what hiding them achieves. (rela-security-reviewer)
severity: minor
resolution: The guide and the gitStatusResponse comment now say this explicitly and no longer present it as a secrecy guarantee; see the significant finding on /_conflicts.
status: addressed
---
