---
id: RR-MMGQX9
type: review-response
title: Add a path-traversal type to the negative tests
finding: The entity type reaches path.Join unvalidated. Type '../relations/x' resolves to relations/xs/<id>.md, outside entities/. The allowlist closes this, which the security section notes. Pin it with a test case (types '../x' and 'a/b') so a later relaxation of the check cannot reopen it.
severity: minor
resolution: 'Plan updated: AC6 tests a path-traversal type string.'
status: addressed
---
