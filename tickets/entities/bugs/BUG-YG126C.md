---
id: BUG-YG126C
type: bug
title: sqlite short-needle search folds case for ASCII only
description: 'sqlitestore matches needles under 3 runes with LIKE, which folds ASCII only, while search.MatchTextFields folds Unicode. The two can disagree on which fields matched, so under visible: a hit whose real match is a hidden field can be admitted. Found in the review of BUG-57A53F (RR-K1JELW).'
priority: medium
status: backlog
---
