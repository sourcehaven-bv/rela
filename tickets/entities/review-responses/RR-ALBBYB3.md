---
id: RR-ALBBYB3
type: review-response
title: 'Code: face toggle on embedded read surfaces'
finding: EntityPreviewModal and SidebarFlyout use hideActions but would show the toggle, navigating away from the host with its query.
severity: significant
resolution: The toggle is not rendered when hideActions is set; the detail panel gets the faces as actions as before.
status: addressed
---
