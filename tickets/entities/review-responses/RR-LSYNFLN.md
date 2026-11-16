---
id: RR-LSYNFLN
type: review-response
title: 'R7 is misattributed: decoder already refuses unknown keys'
finding: '~/bin/rela predates flows.yaml support (#2104); a binary between #2104 and #2350 fails boot on the unknown triggers key because strictDecode sets KnownFields(true) (internal/flowcfg/load.go:196). Only a binary without flows.yaml support is silent; a stale binary fails flow validate instead of printing a count.'
severity: significant
resolution: 'Plan R7 rewritten: silent only for pre-#2104 binaries; strict-decoding follow-up dropped; CLAUDE.md note says flow validate must succeed and report 13 triggers; decision entity records that only a project version floor (none exists) could close the window.'
status: addressed
---
