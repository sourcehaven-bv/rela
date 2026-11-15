---
id: RR-82JYZL0
type: review-response
title: Template language undefined
finding: id and template values need a named, bounded engine.
severity: significant
resolution: Templates use the ingest {{ref}} interpolation (ingest.Interpolate) with refs as paths. Every id is checked with entity.ValidateID.
status: addressed
---
