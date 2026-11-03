---
id: RR-0GH0RM
type: review-response
title: Docs do not say which principal renders or who can read the file
finding: In automations the principal is the run_as identity, not the user who saved. The stored file is readable by every reader of the target entity. A broad run_as is a confused-deputy path that the docs did not mention.
severity: significant
resolution: The Lua guide and docs/transforms.md now state the principal per runtime and that the file's audience is the target entity's readers, and advise a run_as whose reads match those readers. The attach-render audit record already carries triggered_by=automation-job:<name>;by=<user>; the guide documents it.
status: addressed
---
