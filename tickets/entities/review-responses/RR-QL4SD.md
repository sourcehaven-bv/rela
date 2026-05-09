---
id: RR-QL4SD
type: review-response
title: 'ARIA listbox: aria-activedescendant + per-row id required'
finding: 'Plan mentions role=listbox / role=option / aria-selected. For screen readers to announce the highlighted option without focus moving from the input, the WAI-ARIA combobox-with-listbox pattern requires the input to carry aria-activedescendant=<id-of-highlighted-option>, and each <li role=option> needs a unique stable id. Without aria-activedescendant, screen-reader users cannot perceive which result is highlighted. Required: in Approach, add aria-activedescendant wiring (input attribute bound to highlighted row''s id) and per-row id generation (e.g., cmdk-option-${entity.id}). Test: stub results, ArrowDown, assert input''s aria-activedescendant matches the second row''s id.'
severity: minor
resolution: 'Plan updated: each <li role=''option''> gets a stable id `cmdk-option-${entity.id}`; the <input> carries aria-activedescendant bound to the highlighted row''s id. Implements the WAI-ARIA combobox-with-listbox pattern. Test added: stub results, ArrowDown, assert input''s aria-activedescendant matches the second row''s id.'
status: addressed
---
