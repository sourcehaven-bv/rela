---
id: RR-E0BFFN
type: review-response
title: Fullscreen leaves body overflow:hidden on teardown
finding: EasyMDE's toggleFullScreen sets document.body.style.overflow='hidden' and only restores on toggle-off. The element's _unmount (toTextArea/cleanup) didn't exit fullscreen, so disconnecting while fullscreen left the host app's page permanently unscrollable with no editor to fix it.
severity: critical
resolution: _unmount now checks cm.getOption('fullScreen') and calls EasyMDE.toggleFullScreen(editor) before teardown. Covered by 'exits fullscreen on teardown' + 'does not toggle when not fullscreen' tests.
status: addressed
---
