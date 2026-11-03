---
id: RR-ARHKYD
type: review-response
title: filechannels max-methods raised to 41 without need
finding: 'internal/channels/filechannels/file.go:140 adds `//plimsoll:max-methods=41` and calls it a required-interface exception. Only the exported count is mandated by channels.Store. The total is a ratchet, and CLAUDE.md says to prefer splitting over raising it. Fix: turn one unexported helper method on Store into a free function, as the directive comment itself says ("Helpers stay free functions"), and keep the cap at 40.'
severity: nit
resolution: readTypes became a free function so the filechannels directive is removed and the cap stays at 40
status: addressed
---
