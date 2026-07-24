---
id: RR-QSP6X2
type: review-response
title: Denied vs transient store fault conflated in search/list/get_relations error handling
finding: ScriptReader.GetEntity returning store.ErrNotFound for a deny is correct and becomes nil in luaGetEntity (verified). But in luaSearch, `continue` on error means a transient store fault silently drops a hit the caller IS allowed to see; luaListEntities/luaGetRelations `break` and return a silently truncated list. All pre-existing and fail-closed in direction — but the new redaction layer makes short results EXPECTED, so the one signal that used to hint at a fault is now indistinguishable from normal policy behavior. Add a slog.Warn on the non-ErrNotFound branch.
severity: minor
resolution: 'rela.search now distinguishes the two: a denied hit (ErrNotFound) is skipped silently as before, anything else logs a slog.Warn naming the id and error. The comment explains why this matters now — redaction makes short results expected, so a genuine fault would otherwise be indistinguishable from policy.'
status: addressed
---
