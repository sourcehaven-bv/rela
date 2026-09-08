---
id: RR-A1CUO8
type: review-response
title: Entity-type allowlist contradicts metamodel.ValidateSchemaName
finding: validateTemplateSegment allowlisted [A-Za-z0-9_-] for the entity TYPE but the metamodel deliberately uses a blocklist because shipped schemas use dashes / dots / spaces / non-ASCII. The check also ran before the empty-variant return so default templates broke for such types.
severity: significant
resolution: Type names now use validateTemplateName — a blocklist mirroring ValidateSchemaName (empty; . and ..; separators; control chars; edge whitespace). The variant keeps the identifier allowlist identical to automation.isValidTemplateName. Tests pin accepted names such as review-response / some property / v1.2 / café.
status: addressed
---
