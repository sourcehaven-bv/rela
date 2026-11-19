---
id: RR-NQL2ORS
type: review-response
title: Top-level builtins was not served on _config
finding: v1.Config carried spaces (and so space builtins) but no top-level Builtins field, while the docs and godoc said _config serves the block.
severity: minor
resolution: v1.Config gains Builtins (pin raised to 23 as its comment prescribes) and handleV1Config fills it. TestSidebarBuiltins_ConfigUnfiltered pins both blocks for a non-holder.
status: addressed
---
