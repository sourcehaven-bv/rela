---
id: RR-B95TD1M
type: review-response
title: rela-desktop and rela-docs silently build without the SPA under -tags headless
finding: Both binaries need the SPA. With the headless tag CheckEmbeddedSPA passes and they would start with no UI.
severity: minor
resolution: 'The premise was wrong: under the headless tag CheckEmbeddedSPA returns an error, so rela-desktop refuses to start and rela-docs refuses screenshot captures. A compile guard was added first and later removed, because the CI tagged-file check vets every package under each tag.'
status: addressed
---
