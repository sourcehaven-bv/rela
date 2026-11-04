---
id: RR-YHE98E6
type: review-response
title: Kept disallowed edges are refused only by the data-entry API
finding: The PATCH refuses a full linkage that keeps an existing disallowed edge while the manager only checks creates; the docs said the check lives in one place.
severity: minor
resolution: DEC-0D8UZX3, CLAUDE.md and api-reference.md now state the data-entry rule for kept edges; TestPatchRelations_KeepingExistingDisallowedEdgeIs422 pins it.
status: addressed
---
