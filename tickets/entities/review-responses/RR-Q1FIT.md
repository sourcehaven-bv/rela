---
id: RR-Q1FIT
type: review-response
title: 'Unknown meta keys: silent acceptance is wrong for closed-schema relation properties'
finding: |-
    Plan: 'Unknown meta keys: per existing convention, accept silently for forward-compat.' Entity properties may be open-schema in some metamodels, but RelationDef.Properties is closed with explicit type definitions in metamodel/types.go. Silently accepting unknown keys means: (1) lands in YAML frontmatter, (2) GET round-trips them, (3) analyze_properties won't catch them (only validates declared properties), (4) data stuck on disk indistinguishable from real schema property until manual cleanup. The 'silent corruption discovered six months later' kind of bug.

    Fix: unknown meta keys → 422 with {pointer: '/relations/<type>/data/<i>/meta/<unknown_key>', message: 'unknown property for relation type X'}. Matches ValidateRelationProperties strictness, prevents schema drift. Forward-compat not a concern: metamodel is checked into the same repo as data; new properties require a metamodel commit.

    If existing properties validator does silently accept unknown keys for entity properties, that's a separate bug to file — not a precedent to extend.
severity: significant
resolution: Unknown meta keys → 422 with structured pointer. RelationDef.Properties is a closed schema. Approach step 9 calls ValidateRelationProperties which already rejects unknown keys (verified via research). Security section documents the strictness.
status: addressed
---
