---
id: RR-02PMQL
type: review-response
title: No guard against future slog.SetDefault tests going parallel
finding: tparallel does not catch the slog.SetDefault-under-parallel class (unlike t.Setenv, which the runtime refuses). The next parallel wave could reintroduce it.
severity: minor
resolution: Both sites now carry NOT-parallel comments naming the rule; the ticket documents the class (slog.SetDefault / env / cwd / time.Local must stay serial). A lint/grep gate was considered and deferred — two known sites, both commented.
status: addressed
---
