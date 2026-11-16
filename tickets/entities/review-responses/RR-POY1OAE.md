---
id: RR-POY1OAE
type: review-response
title: Data race on prefs.Window
finding: saveWindowState writes d.prefs.Window from the hook while other goroutines read prefs.
severity: minor
reason: Preferences is unsynchronized throughout rela-desktop; a narrow lock here risks a deadlock with NewWithOptions calling InvokeSync. Needs its own change to give Preferences one owner.
status: deferred
---
