---
id: RR-6MG8WAC
type: review-response
title: '[security] A fixture file could read .rela/secrets.yaml'
finding: '[security] fixtureFile accepted any local, symlink-free path, including .rela/secrets.yaml, whose content could reach a failure message or a CI log. The check must also hold on a case-insensitive file system.'
severity: minor
resolution: 'Paths whose first element is .rela, compared case-insensitively, are refused. Tests: cache dir and cache dir upper case.'
status: addressed
---
