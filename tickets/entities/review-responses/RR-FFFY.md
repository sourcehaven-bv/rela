---
id: RR-FFFY
type: review-response
title: Plan misses `handleOpenURL` and `handleGitSync` in the endpoint inventory
finding: 'Verification surfaced endpoints the original plan didn''t list: handleOpenURL (POST /api/open-url), handleGitSync (POST /api/git/sync), handleToggleCheckbox (POST /api/toggle-checkbox), handleV1ConflictResolve (POST /api/v1/_conflicts/), handleCommandCancel (POST /api/command-cancel/{execID}). Each is a state-changing endpoint that needs the same Origin/Host gate. handleOpenURL is particularly interesting: if it shells out to `open <url>` without scheme validation, an attacker could pass `file:///etc/passwd` for another file disclosure, or a `javascript:` URL on systems where the default handler executes it.'
severity: significant
resolution: 'Plan updated: explicit endpoint inventory added to the planning doc (every POST/PUT/PATCH/DELETE handler in internal/dataentry plus the GET-mutating ones). handleOpenURL specifically gets a scheme allowlist (http, https, mailto only) added as part of this ticket since it''s the same class of bug as handleOpenFile.'
status: addressed
---
