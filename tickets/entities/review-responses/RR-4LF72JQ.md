---
id: RR-4LF72JQ
type: review-response
title: rela remote pull gets no project services
finding: requiresProject did not list remote, so kong had no WorkspaceProvider or IdentityBinder binding for rela remote pull and the command failed before pulling. identityWrites also missed remote pull, so the --run-as notice was not printed.
severity: critical
resolution: Added remote to requiresProject and remote pull to identityWrites. TestRequiresProject_CoversEveryServiceTaker walks the kong tree and fails when a command takes a project-only binding without requiresProject; it fails without the fix.
status: addressed
---
