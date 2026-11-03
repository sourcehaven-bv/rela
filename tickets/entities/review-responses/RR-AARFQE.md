---
id: RR-AARFQE
type: review-response
title: Plimsoll directives and a type assertion back channel
finding: TaggedEntities pushed ScriptReader and UnrestrictedReader to 21 exported methods with new directives and the reader asserted its tag reader to a second interface.
severity: significant
resolution: WithVersionTags merged into WithHistory(h tags) and TaggedEntities joined VersionTagReader. Both readers are back at 20 exported methods and the directives are gone. The Lua binding keeps its reader assertion because the history bindings use the same pattern for optional reads.
status: addressed
---
