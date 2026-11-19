---
id: RR-C328STD
type: review-response
title: Checkboxes can flash before the sidebar loads
finding: listButtons starts true, so a list may briefly show checkboxes until /_sidebar answers.
severity: minor
reason: Same timing as header_buttons, which the SPA accepts; an older server sends no list_buttons and must keep showing the button.
status: wont-fix
---
