---
id: RR-Z9FDWK
type: review-response
title: ai.yaml accepted an inline API key
finding: ai.ParseConfig ignores unknown keys, so an api_key typed into the AI settings was stored in the document.
severity: significant
resolution: checkHostFile decodes ai.yaml strictly (KnownFields) and points to the ai_api_key secret. Tested.
status: addressed
---
