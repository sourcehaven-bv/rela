---
id: RR-T4ZEY3
type: review-response
title: Zero-height rail lets the button hang past the end of the body
finding: Sticky limit ignored the button's height, so at the end of the body the pencil overlapped the next section by its height and could take its clicks.
severity: minor
resolution: 'Fixed. Rail is as tall as the button (--rl-inline-edit-button-size) and gives it back with a negative margin; the rail is pointer-events: none so it does not block selecting the first line. Also fixed the button being stretched to 10px tall by the zero-height flex rail.'
status: addressed
---
