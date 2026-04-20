---
id: RR-242DF
type: review-response
title: ensureKeyGitignored does not warn when user's --identity source is inside .rela/
finding: 'keys init --identity <path> copies user-supplied identity. If the source path is inside the project tree (common: users put it in .rela/ because that''s how rela has worked), we ingest it but leave the original in the synced tree. The whole point of the ticket is that the key shouldn''t live there.'
severity: minor
resolution: 'Post-copy check: if --identity source path is inside projectRoot, emit a visible warning suggesting the user delete the source file now that it has been copied to the user-state location. Does not delete automatically.'
status: addressed
---
