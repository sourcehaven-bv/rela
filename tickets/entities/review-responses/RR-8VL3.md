---
id: RR-8VL3
type: review-response
title: 'F6: BaseURL validation allows credentials via query string'
finding: Validator correctly rejected https://user:secret@host but did NOT reject https://host/v1?api_key=secret or ?token=secret, which some legacy providers (notably old Azure and some compat layers) actually use. That URL would then end up in every logRequestStart line via base_url=, leaking the API key in plaintext to logs. The redactKey path didn't cover this because resolveAPIKey only knows about env-var keys, not embedded query-string ones.
severity: significant
resolution: Config.Validate now rejects any base_url with a non-empty RawQuery or Fragment. New TestLoadConfig_BaseURLWithQueryString and TestLoadConfig_BaseURLWithFragment lock it in. Documented inline that auth must come through api_key_env only.
status: addressed
---
