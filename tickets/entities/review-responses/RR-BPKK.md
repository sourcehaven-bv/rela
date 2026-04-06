---
id: RR-BPKK
type: review-response
title: 'Hex parsing must handle bare hex without # prefix'
finding: Lospec hex downloads are bare hex values like 'ddcf99' without '#'. The parser must accept and normalize both '#ddcf99' and 'ddcf99' to '#ddcf99'. The plan only mentions 'hex list' without specifying prefix handling.
severity: minor
resolution: 'Parser accepts both bare hex (ddcf99) and prefixed (#ddcf99), normalizes to #rrggbb'
status: addressed
---
