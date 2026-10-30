---
id: RR-3MALUVU
type: review-response
title: Reload warning can never fire
finding: The warning ran per generation and the guide promised it on each reload, but the only rebuild path is a Configure save, which validateConfigEditing allows only with acl.yaml and an identity source, so the condition is always false there.
severity: significant
resolution: The check moved to newServer (startup only) with a comment naming why; the guide says 'at startup'.
status: addressed
---
