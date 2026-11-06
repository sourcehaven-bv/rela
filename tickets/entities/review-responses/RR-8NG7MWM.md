---
id: RR-8NG7MWM
type: review-response
title: Non-saver that leaves drops its unsaved document
finding: stop, expire, fallBack and staleBuild flush the editor, but for a non-saver input only arms the fallback timer, which end() clears. commitImmediately has nothing pending, so the client's last edits are lost when the saver is gone.
severity: significant
resolution: 'saveHeld runs on stop, expire, fall-back and stale-build for every client, saver or not. Test: useBodyCollab ''saves what a non-saver holds when it leaves''.'
status: addressed
---
