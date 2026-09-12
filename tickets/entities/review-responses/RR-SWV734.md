---
id: RR-SWV734
type: review-response
title: Page-limit truncation could hide a match and file a duplicate
finding: The exact-title match is client-side over a server-side page capped at `--limit 200`. Once 200 open `fuzz-failure` issues exist, a genuine match can sit outside the fetched page, so the step would file a duplicate — degrading quietly, the same way the swallowed-lookup bug did.
severity: significant
resolution: 'Bounded and asserted rather than redesigned. The sweep discovers 52 fuzz targets, so at most 52 distinct titles can exist and the 200 cap is unreachable today. The listing is now fetched once and its length compared against ISSUE_PAGE_LIMIT; hitting the cap prints a warning telling the operator dedup may file duplicates and to triage the backlog. Verified with a stub returning a full 200-issue page (warning fires, issue still files) and a normal page (no warning, correct comment/create). Accepted cost: this reintroduces a standalone `jq` dependency, confirmed with the user.'
status: addressed
---

Found by cranky-code-reviewer on the TKT-8LZGME diff.

The reviewer suggested `--search "$title in:title"` as a server-side pre-filter.
Not taken: GitHub's search is fuzzy and would need the exact client-side
equality kept anyway as the authority, so it adds a second API dependency
without removing the cap. Asserting the bound is the smaller change for a branch
that cannot currently fire.

Dependency note: `jq` is preinstalled on `ubuntu-latest` but no other workflow
in this repo uses it standalone. The user chose to accept it in exchange for the
explicit guard.
