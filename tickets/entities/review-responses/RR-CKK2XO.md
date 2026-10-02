---
id: RR-CKK2XO
type: review-response
title: sync on a comment-only file drops the comments
finding: yaml.v3 parses a file holding only comments to an empty node, so sync rebuilt the root and lost the operator's comments.
severity: minor
resolution: documentFor keeps the comment lines of the raw text above rela's header. Pinned by TestSync_CommentOnlyFileKeepsComments.
status: addressed
---
