---
id: RR-E0JRND
type: review-response
title: e2e files were reformatted wholesale by a stray Prettier run
finding: e2e/ has its own package.json and no .prettierrc, so running the repo's Prettier over it converted
  single quotes to double throughout and rewrapped calls. 241 changed lines in apps.spec.ts where roughly
  40 were substantive, burying the new tests and making git blame useless on a security-relevant spec.
severity: nit
resolution: 'Reverted all three e2e files to HEAD and reapplied the changes by hand in the original style.
  They are now pure additions: apps.spec.ts +129/-0, app-host.page.ts +104/-0, fixtures.ts +29/-1.'
status: addressed
---
