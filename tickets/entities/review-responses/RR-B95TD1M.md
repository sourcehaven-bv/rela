---
id: RR-B95TD1M
type: review-response
title: rela-desktop and rela-docs silently build without the SPA under -tags headless
finding: Both binaries need the SPA. With the headless tag CheckEmbeddedSPA passes and they would start with no UI.
severity: minor
resolution: Add a headless-tagged file to cmd/rela-desktop and cmd/rela-docs that fails compilation with a clear identifier name.
status: addressed
---
