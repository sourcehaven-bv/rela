---
id: RR-1GNK1B
type: review-response
title: 'Design: postgres faced fixture cannot read seed files'
finding: History is pgstore-only and the postgres server starts empty; the base postgresTest also does not pass RELA_DATAENTRY_USER. A history spec on the faced fixture would find no POL-1 and run as an unassigned principal.
severity: significant
resolution: facedPgTest overrides serverUrl to pass both RELA_DATABASE_URL and the chosen user, and the history spec seeds its policy through facedApi.createPolicy before asserting.
status: addressed
---

History is pgstore-only and the postgres server starts empty; the base
postgresTest also does not pass RELA_DATAENTRY_USER. A history spec on the faced
fixture would find no POL-1 and run as an unassigned principal.
