---
id: RR-PXMNR83
type: review-response
title: Header menu button opens an empty drawer while presenting
finding: The app header's hamburger stayed visible in present mode and opened an empty navigation drawer.
severity: minor
resolution: 'App.vue hides it while presenting and RlAppShell sets --rl-nav-toggle-display: none under rl-app-shell--presenting.'
status: addressed
---
