---
id: RR-WMG2
type: review-response
title: Modal z-index inside EasyMDE fullscreen — relies on unspecified DOM order
finding: Plan says 'modal renders on top (z-index already 9999 via the existing fullscreen CSS; verified in browser).' EasyMDE fullscreen mounts the editor at z-index 9999. CommandPaletteModal teleports to `<body>` with its own overlay; the relative stacking depends on DOM order AND z-index. Without explicit z-index on the picker overlay, fullscreen mode may put the editor on top of the picker (same z-index, later element wins). The plan should specify an explicit z-index for the picker overlay above 9999 (e.g. 10000) AND verify by toggling fullscreen in the e2e.
severity: significant
resolution: 'Plan §Approach §1: explicit ''z-index: 10000'' on overlay (+1 above EasyMDE''s 9999 fullscreen layer); CSS comment documents the relationship. AC 9 + e2e case toggles fullscreen before opening the picker.'
status: addressed
---
