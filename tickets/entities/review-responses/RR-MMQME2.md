---
id: RR-MMQME2
type: review-response
title: 'Review nits: prototype lookup, hard-coded en, json tag, vite comment'
finding: API_ERROR_KEYS lookup reached Object.prototype for code constructor; the English check was a literal; Language had a json tag its siblings lack; the vite flag comment was wrong.
severity: nit
resolution: apiErrorKey uses an own-property check (tested); SERVER_MESSAGE_LANGUAGE constant; json tag dropped; comment corrected.
status: addressed
---
