---
id: RR-MZO5R2
type: review-response
title: Pass the bare id and world; keep the raw reload
finding: MentionedBy needs result.Entry.ID and the request world; redacted page items must not be fed to the redacting view path.
severity: minor
resolution: collectBacklinks passes the bare entry id and the view world to the resolver and reloads raw headers through loadViewEntities.
status: addressed
---
