---
id: RR-7P3O
type: review-response
title: Manifest accepts logo extensions we will never serve
finding: theme.go::validateLogoEntryName + test 'logo with multiple dots' — Manifest validation accepts `logo.tar.gz` even though the sniffed-mime → ext path will only ever store .png/.jpeg/.svg/.webp. Either reject manifest-time extensions outside allowedLogoExts (matches what we persist) or document explicitly that manifest.logo is a lookup-key string thrown away after sniff.
severity: minor
resolution: validateLogoEntryName now rejects extensions outside the allowlist {png, jpeg, jpg, svg, webp}. Test 'logo with multiple dots' (logo.tar.gz) was moved from the accept matrix to the reject matrix with `unsupported extension`. New tests cover jpg alias and gif rejection.
status: addressed
---
