---
id: RR-WZYSHB
type: review-response
title: Hiding conflict paths in /_git/status was claimed as secrecy but /_conflicts still serves them
finding: gitStatusResponse dropped ConflictFiles for non-syncers and its comment and the docs said this was because entity existence is secret. GET /api/v1/_conflicts and /api/v1/_conflicts/{path} still return the paths, entity ids and raw content to every principal (BUG-T6HNKS), so the documented guarantee was false. (cranky-code-reviewer)
severity: significant
resolution: The comment on gitStatusResponse and the git:sync section of the ACL guide no longer claim secrecy. They say the paths go only to a principal who may sync, that a git:sync holder sees paths of entities they may not read, and that /_conflicts still lists them to everyone (BUG-T6HNKS). Gating /_conflicts is BUG-T6HNKS's fix and stays out of this one.
status: addressed
---
