---
id: RR-2KEC8P
type: review-response
title: Duplicate busy= should follow the comment_counts convention
finding: AC 4 treats a repeated busy as absent, but comment_counts reads the first value through queryGet (internal/dataentry/comment_counts.go:24), and live list paths accept repeated keys (internal/dataentry/livepath.go:131-135). The plan should use queryGet for consistency, or say why busy differs.
severity: nit
resolution: The parameter is read with queryGet, first value wins, as comment_counts is; AC 4 is updated.
status: addressed
---
