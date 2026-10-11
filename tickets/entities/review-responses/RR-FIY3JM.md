---
id: RR-FIY3JM
type: review-response
title: _piles routes refused in non-default worlds
finding: worldCapablePath denies underscore routes not on its list, so _piles?world=x is 422 while _position works; panel counts and the scope total would disagree.
severity: significant
resolution: 'Plan: admit _piles and _piles/{id} (and its _export) in worldCapablePath with a call-site justification; extend viewworld_guard_test. Test: panel count equals the _position total in a non-default world.'
status: addressed
---
