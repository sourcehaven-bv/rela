---
id: RR-4OMU
type: review-response
title: Bump multipart envelope budget to avoid borderline 413s
finding: MaxBytesReader is capped at MaxUserLogoBytes + 4 KiB envelope (handlers_theme.go::maxLogoUploadBytes). 4 KiB is empirical and could clip a legitimate browser upload with a long UTF-8 filename or extra form fields. Either bump to ~16 KiB headroom or drop the body cap entirely and rely on the post-parse len(bytes) > MaxUserLogoBytes check (with the body cap set to something generous like 1 MiB).
severity: significant
resolution: Bumped envelope headroom from 4 KiB to 16 KiB (handlers_theme.go::maxLogoUploadBytes). Existing TestThemeLogo_ExactlyAtLimit + TestThemeLogo_TooLarge both still pass.
status: addressed
---
