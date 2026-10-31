---
id: RR-BP7UYXD
type: review-response
title: Planned note design tokens were not added
finding: The plan named --rl-color-note-* tokens; the code reuses --rl-tag-yellow-bg and literal shadows and sizes.
severity: minor
reason: 'Change of plan: the --rl-tag-* tokens already have dark-mode values and pass the library''s theme and contrast checks; new tokens would duplicate them. The literal sizes are layout values local to the wall styles.'
status: wont-fix
---
