---
id: RR-2DGKOU3
type: review-response
title: Configure problem text contradicts its test comment
finding: cmd/rela-server/configure.go:115 prefixes data-entry.yaml while the description also names it; the test comment said the file is named once.
severity: minor
resolution: Test comment rewritten to describe what it pins; the description text comes from the migration and stays as is.
status: addressed
---
