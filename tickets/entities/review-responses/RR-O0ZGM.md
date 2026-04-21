---
id: RR-O0ZGM
type: review-response
title: validateCreateIDOpts did not trim whitespace, leading to misleading errors
finding: Inputs like '  TAG-foo' (leading space) failed strings.HasPrefix because of the space, so the user got a 'must start with...' error that was technically true but misleading; the real problem was the whitespace. The Vue side (`<input v-model="manualId">`) also doesn't trim. Copy-pasting an ID with a stray newline gave bewildering errors.
severity: significant
resolution: validateCreateIDOpts now trims id and prefix at the top with strings.TrimSpace; the v1 + legacy handlers also trim req.ID/req.Prefix before calling. Added test rows 'manual prefixed, whitespace-only id treated as empty' and 'short, whitespace prefix treated as empty' to TestValidateCreateIDOpts.
status: addressed
---
