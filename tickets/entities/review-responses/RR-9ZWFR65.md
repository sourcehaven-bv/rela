---
id: RR-9ZWFR65
type: review-response
title: Status and sync routes pass through unknown paths that can name hidden entities
finding: readableConflictPaths gated only paths under entities/ and relations/ and listed every other path. attachments/<id>/... and other per-entity state paths name an entity, so a sync holder learned that a hidden entity exists.
severity: significant
resolution: The filter now fails closed. An attachment path is kept only when its owner id is readable at some face (one batch). Config files and config directories are on an explicit allowlist. Every other path is left out. TestGitStatusResponse_ConflictFiles covers a hidden and a visible attachment, a deleted owner, config files and unknown paths.
status: addressed
---
