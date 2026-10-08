---
id: RR-TII45K
type: review-response
title: '[security] Docs understate what a shared clamd socket gives the PDF converters'
finding: 'With one host-wide list the recommended value makes clamd.conf readable via \\input and the clamd socket connectable from every export converter; clamd''s local protocol accepts SHUTDOWN, RELOAD and SCAN <path>. Previously only the attachment runner had these binds (RR-61YI8G). Docs only: state the exposure and that it widens compared with the scan-only binding.'
severity: minor
resolution: attachment-security guide now states that export converters can read clamd.conf and connect to the clamd socket, that this is new compared with the scan-only binding, what a clamd client can do (SHUTDOWN, SCAN <path>), and to run clamd under a restarting service manager; the Upgrading note points at it.
status: addressed
---
