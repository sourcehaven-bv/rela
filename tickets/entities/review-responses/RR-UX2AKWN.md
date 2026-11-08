---
id: RR-UX2AKWN
type: review-response
title: Relation endpoint scope must be stable
finding: Child relation spans with narrower endpoint sets than the parent can lose rows whose endpoint type is outside the current From/To. Use one endpoint rule at every level.
severity: minor
resolution: Every relation range sends all scope entity types as its endpoint scope on both sides; documented on Scope and in the CLI docs.
status: addressed
---
