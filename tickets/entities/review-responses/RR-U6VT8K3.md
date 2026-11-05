---
id: RR-U6VT8K3
type: review-response
title: Write check judged the resolved side's type and edge, not the file's
finding: The entity row type came from the resolved frontmatter and a relation was checked on the resolved edge, either of which can differ from the file the write replaces.
severity: significant
resolution: resolvedMatchesFile refuses (422 conflict_identity_mismatch) unless the resolved id and type, or the edge, equal the file's; the entity is authorized as the file's row. TestV1ConflictResolve_FileIdentity pins it.
status: addressed
---
