---
id: RR-77QXF2
type: review-response
title: mcp and cli importing internal/channels just for State is avoidable
finding: Section 0 adds the storage-abstraction package to the mcp and cli arch-lint deps only for channels.State, then needs a grep test to keep channels.Store out. Have realtime take or alias the state (for example realtime.State or a string), so arch-lint enforces the boundary and no grep test is needed.
severity: minor
resolution: realtime exports a State type, so mcp and cli do not import channels. arch-lint enforces it and the grep test is dropped.
status: addressed
---
