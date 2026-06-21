---
id: RR-9A3YOQ
type: review-response
title: appEntryContentType uses mime.TypeByExtension — common app extensions ARE deterministic (built-in)
finding: 'DOWNGRADED from significant after checking Go''s mime package. The reviewer claimed .js could resolve to application/octet-stream on a box where it''s unregistered, breaking the app. FALSE for the extensions that matter: .js .mjs .css .html .json .svg .wasm .png .jpg .gif .webp .zip etc. are ALL in Go''s hardcoded builtinTypesLower (mime/type.go:64-129), set unconditionally at init BEFORE the OS file is consulted (initMime: setMimeTypes(builtin) then osInitMime augments). So an app''s app.js/style.css get correct types regardless of the deploy box''s /etc/mime.types. Residual (theoretical only): a pathological OS mime file could OVERRIDE a built-in (osInitMime overwrites the map) — e.g. a hostile /etc/mime.types mapping .js→text/plain — but that''s a self-inflicted misconfiguration, not a real scenario. NET: nit. An explicit allow-map would be marginally nicer (immune even to the override case + self-documenting), but it''s a polish, not a correctness fix. Worth doing opportunistically; not required.'
severity: nit
resolution: 'Replaced mime.TypeByExtension with an explicit appContentTypes allow-map (deterministic, case-insensitive, immune to the OS-mime-override edge), unknown extensions → application/octet-stream. Test: TestAppEntryContentType covers js/mjs(case)/css/svg/woff2/json + unknown + no-extension fallback.'
status: addressed
---
