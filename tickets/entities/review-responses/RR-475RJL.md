---
id: RR-475RJL
type: review-response
title: Glob negation and dotfile semantics differ from Claude Code
finding: '[!x] was passed through as a class including !; * matches dotfiles unlike picomatch.'
severity: nit
resolution: '[!x] translates to a negated class with a test; the dotfile difference is documented because no rule glob names a dotfile.'
status: addressed
---
