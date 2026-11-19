---
id: RR-Y7HYKT1
type: review-response
title: TS config type lacks entity_header
finding: The /_config TypeScript type did not declare entity_header although the server serves it.
severity: minor
resolution: Added entity_header (since renamed header_buttons) with ChromeToggle fields to the Config interface.
status: addressed
---
