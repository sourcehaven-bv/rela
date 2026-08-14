---
id: RR-IOZI72
type: review-response
title: clear_when_hidden enum has no allowlist validation site
finding: 'The plan names three values (no|yes|confirm) but does not say where the allowlist lives, and no validation exists today. conditionlint.go:57-76 checks only the four visible_when expression sites. A typo such as clear_when_hidden: never — or the plausible off/false — errors nowhere. yaml.v3 decodes bare ''off'' as the literal string "off", so it reaches the frontend intact. Today the ===''yes''/===''confirm'' comparisons would all miss and it falls through to the safe default, but the moment someone writes the comparison as !==''no'', ''off'' silently means CLEAR. Needs allowlist validation in validateFormField (validate.go:276) alongside the existing transitions check, plus a frontend union type ''no''|''yes''|''confirm'' rather than string.'
severity: significant
resolution: |-
    Implemented. Allowlist lives in internal/dataentryconfig/config.go (ValidClearWhenHidden, with named constants) and is enforced in validateFormField (validate.go), alongside the existing transitions check. Frontend mirrors it as a union type ClearWhenHidden = 'no' | 'yes' | 'confirm' in types/config.ts, not string.

    Also added a guard for clear_when_hidden set WITHOUT visible_when (it could never fire) — a config mistake worth catching at author time.

    Tests: TestValidateConfig_ClearWhenHiddenAllowlist covers all three valid values plus the plausible-typo rejections (off, false, true, confrim, never). TestFormField_ClearWhenHiddenYAMLScalars pins the YAML-scalar behavior directly: bare no/yes decode to the literal strings "no"/"yes" under yaml.v3's 1.2 core schema and land in the allowlist as written, so the boolean footgun does not bite. Verified experimentally before relying on it.
status: addressed
---
