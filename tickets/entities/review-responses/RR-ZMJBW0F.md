---
id: RR-ZMJBW0F
type: review-response
title: Sentinel message assumes the block is in the body
finding: The check scans the whole document so the fix named in the message may not fit a frontmatter block.
severity: nit
reason: markdown.KeyNode quotes a key that begins with the start marker so frontmatter cannot form a block on write; only content reaches the check and the message names the fix for it.
status: wont-fix
---
