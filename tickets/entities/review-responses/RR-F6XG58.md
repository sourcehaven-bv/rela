---
id: RR-F6XG58
type: review-response
title: PlantUMLServerURL not validated at config load
finding: 'renderPlantUMLDiagrams sets img.src from schemaStore.app.plantuml_server_url verbatim, and dataentryconfig.ValidateConfig never validated App.PlantUMLServerURL. This is a config-validation gap, not a security hole: pointing the URL at a malicious host requires write access to data-entry.yaml, which is a trusted checked-in file (anyone who can edit it can already do worse via command:/script: documents). The real value of validation is catching misconfiguration — a typo, a wrong scheme, or pasting the public plantuml.com server unintentionally — and failing fast at startup. (Original review framed this as a critical ''exfil primitive''; downgraded to minor after analysis: the <img src> exfil mechanism is real but there is nothing to enforce against beyond the existing data-entry.yaml trust boundary.) Fix: validate at config load (absolute http/https + host); a server-side proxy that hides the render host from the browser is the genuinely security-shaped option, deferred as RR-21O6D4.'
severity: minor
resolution: 'Added validateApp in internal/dataentryconfig/validate.go (rejects non-http(s) scheme, missing host, malformed URL), wired into ValidateConfig. Added client-side defense-in-depth: plantUMLImageURL parses the URL and returns null for non-http(s) schemes, so renderPlantUMLDiagrams leaves the block as plain code. Tests: TestValidateApp_PlantUMLServerURL (9 cases) + frontend ''unsafe scheme'' no-op test.'
status: addressed
---
