---
id: RR-ZSJ99
type: review-response
title: Backend AutoSave field needs explicit json:auto_save tag; TS type should be auto_save? to match snake_case convention
finding: |-
    Plan: 'AutoSave bool yaml:"auto_save"' on Go struct + 'autoSave?: boolean' on TS type. Without explicit json: tag, Go encodes the field as PascalCase 'AutoSave'. Existing TS FormConfig uses snake_case (target_type, allow_create) matching explicit json: tags on related fields. Mixing PascalCase and snake_case is a needless papercut.

    Fix: AutoSave bool `yaml:"auto_save" json:"auto_save,omitempty"` on Go; auto_save?: boolean on TS, matching existing convention.
severity: significant
resolution: 'Plan now specifies: Go field `AutoSave bool ` + `yaml:"auto_save" json:"auto_save,omitempty"` (explicit json tag). TS type uses `auto_save?: boolean` (snake_case, matching FormConfig convention). AC #17 covers via Go test on the V1Config response.'
status: addressed
---
