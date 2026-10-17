---
id: BUG-YG126C
type: bug
title: sqlite short-needle search folds case for ASCII only
description: 'sqlitestore matches needles under 3 runes with LIKE, which folds ASCII only, while search.MatchTextFields folds Unicode. The two can disagree on which fields matched, so under visible: a hit whose real match is a hidden field can be admitted. Found in the review of BUG-57A53F (RR-K1JELW).'
priority: medium
effort: m
why1: sqlitestore matched needles under 3 runes with LIKE (ASCII-only fold) and longer ones with an FTS5 trigram index that folds case with its own Unicode tables, while search.MatchTextFields folds with Go's strings.ToLower.
why2: The indexed text was built by SQL triggers from the raw row, so no fold Go applies could reach it.
why3: The rule that SQL and Go must fold alike was written for pgstore's lower() and checked only there.
why4: The field scope conformance cases used Latin letters that every fold agrees on, so no backend was tested with a needle where the folds differ.
why5: A rule about matching was enforced per backend by review rather than by one differential conformance case run on every backend.
prevention: 'MEAS-SQLITE-SHORT-NEEDLE-FOLD: BackendFoldsLikeGo in storetest.RunFieldScopeSearchTests runs a short and a long non-ASCII needle on every backend. sqlitestore and pgstore share search.IndexText, so the indexed text is folded by Go in both.'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---
