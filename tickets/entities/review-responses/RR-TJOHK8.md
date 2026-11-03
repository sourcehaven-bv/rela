---
id: RR-TJOHK8
type: review-response
title: Unique temp names remove self-healing; leftovers pile up where nothing sweeps
finding: 'Today a crashed write leaves path.tmp, and the next write of that path reuses and replaces it, so there is at most one leftover per file. With CreateTemp names every crash leaves a new file. The plan only adds a sweep for entities/ and relations/ (fsstore.cleanupTempFiles). SafeFS also writes .rela state (state.FSKV, privateFile), comment threads (filecomments), channel segments and metadata (filechannels), migration state (filemigstate), schema/config files (projectsetup, classification). None of those directories is swept, so leftovers grow without bound there. They also cause harm: filechannels pruneEmpty never removes a directory that holds a leftover; fsimport classifyRela copies any file under the theme/ and migration/ prefixes into the target state KV, so an orphan like migration/x.json.123456.tmp becomes a bogus state key. Fix: put one predicate in storage (for example IsAtomicTempName plus a stale-after constant), use it in fsstore.cleanupTempFiles, and either sweep the parent directory of each SafeFS write lazily (remove stale siblings matching base.*.tmp) or add the sweep to the other file-backed stores. Make fsimport skip temp names. At minimum, list this in Risks with the directories affected.'
severity: significant
resolution: 'Plan updated: one shared temp-file rule in internal/storage (name builder, predicate for the new, old .tmp and .new forms, stale-after rule) used by the fsstore sweep, rela gc --temp-files and an fsimport skip; the remaining leftover cases are recorded under Risks.'
status: addressed
---
